// Package trash implements the board's one recoverable mutation (FR6): moving a
// work folder to .gogo/trash/<ts>-<slug>/ instead of deleting it. It NEVER
// removes the source first — os.Rename, with a copy-then-remove fallback across
// devices — so a delete is always reversible via List + Restore. This is the
// single place the gogo CLI writes OUTSIDE .gogo/resources/ (documented in
// docs/cli-contract.md).
package trash

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// DirName is the trash root under .gogo/.
const DirName = "trash"

// tsLayout is a filesystem-safe, sortable compact UTC timestamp (no ':' — it is
// illegal on some filesystems and awkward in a path). It has NO '-', so the
// first '-' in an entry name always separates the timestamp from the slug.
const tsLayout = "20060102T150405Z"

// featurePrefix is the work-folder name prefix a slug is restored under.
const featurePrefix = "feature-"

// Entry is one .gogo/trash/<base>/ folder.
type Entry struct {
	Base     string // "<ts>-<slug>" folder name (the restore handle)
	Dir      string // absolute path
	TSRaw    string // raw compact timestamp token
	When     string // "YYYY-MM-DD HH:MM:SS" when TSRaw parses, else TSRaw
	Slug     string // original feature slug
	OrigName string // "feature-<slug>" — the .gogo/work name to restore to
	Phase    string // original phase from the trashed state.md ("-" if unknown)
	Status   string // original status from the trashed state.md ("-" if unknown)
}

// Name builds the trash entry folder name "<compact-ts>-<slug>".
func Name(ts time.Time, slug string) string {
	return ts.UTC().Format(tsLayout) + "-" + slug
}

// trashDir is <root>/.gogo/trash.
func trashDir(root string) string { return filepath.Join(root, ".gogo", DirName) }

// slugFromFeatureDir strips the leading "feature-" from a work-folder base name.
func slugFromFeatureDir(featureDir string) string {
	return strings.TrimPrefix(filepath.Base(featureDir), featurePrefix)
}

// requireUnderWork rejects a source path that does not resolve under
// <root>/.gogo/work/ — the package-boundary guard that keeps the append-only
// changelog (and anything else outside .gogo/work/) undeletable (REV-010).
func requireUnderWork(root, featureDir string) error {
	work := filepath.Join(root, ".gogo", "work")
	absWork, err := filepath.Abs(work)
	if err != nil {
		return err
	}
	absFeature, err := filepath.Abs(featureDir)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absWork, absFeature)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("refusing to trash %s: only .gogo/work features are deletable (not under %s)", featureDir, work)
	}
	return nil
}

// MoveToTrash moves a .gogo/work/feature-<slug>/ folder into
// .gogo/trash/<ts>-<slug>/ and returns the created Entry. The source is never
// removed before the destination is in place (os.Rename is atomic; the
// cross-device fallback copies first, then removes).
func MoveToTrash(root, featureDir string) (Entry, error) {
	info, err := os.Stat(featureDir)
	if err != nil {
		return Entry{}, fmt.Errorf("nothing to delete at %s: %w", featureDir, err)
	}
	if !info.IsDir() {
		return Entry{}, fmt.Errorf("%s is not a folder", featureDir)
	}
	// Package-level defense in depth (the changelog is an append-only archive that
	// must NEVER be trashable — D3/FR6). The TUI already bounces a changelog card,
	// but this belt-and-suspenders makes the invariant hold at the package boundary
	// for EVERY caller: refuse any source that does not resolve under
	// <root>/.gogo/work/ (so a .gogo/changelog/… path can never be moved).
	if err := requireUnderWork(root, featureDir); err != nil {
		return Entry{}, err
	}
	slug := slugFromFeatureDir(featureDir)
	dir := trashDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Entry{}, err
	}
	base := Name(time.Now(), slug)
	dest := filepath.Join(dir, base)
	// A same-second re-delete of a re-created slug would collide — disambiguate.
	dest = uniqueDest(dest)
	base = filepath.Base(dest)
	if err := moveDir(featureDir, dest); err != nil {
		return Entry{}, err
	}
	return entryFor(dir, base), nil
}

// List returns the trash entries, newest first (base name sorts chronologically
// because the timestamp is a fixed-width compact prefix). Absent trash dir →
// empty, never an error.
func List(root string) ([]Entry, error) {
	dir := trashDir(root)
	des, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Entry
	for _, de := range des {
		if !de.IsDir() {
			continue
		}
		out = append(out, entryFor(dir, de.Name()))
	}
	// Newest first: reverse-lexicographic on the compact-ts-prefixed base.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// Restore moves a trash entry back to .gogo/work/feature-<slug>/. It REFUSES
// when a folder with that name already exists (never clobbers live work) and
// when the entry is missing.
func Restore(root, base string) (string, error) {
	base = strings.TrimSuffix(base, "/")
	src := filepath.Join(trashDir(root), base)
	if info, err := os.Stat(src); err != nil || !info.IsDir() {
		return "", fmt.Errorf("no trash entry %q", base)
	}
	_, slug := parseBase(base)
	if slug == "" {
		return "", fmt.Errorf("cannot derive a slug from entry %q", base)
	}
	dest := filepath.Join(root, ".gogo", "work", featurePrefix+slug)
	if _, err := os.Stat(dest); err == nil {
		return "", fmt.Errorf("refusing to restore: %s already exists in .gogo/work", featurePrefix+slug)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	if err := moveDir(src, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// entryFor builds an Entry from a trash base name, enriching it with the
// original phase/status read leniently from the trashed state.md (best-effort).
func entryFor(dir, base string) Entry {
	ts, slug := parseBase(base)
	e := Entry{
		Base:     base,
		Dir:      filepath.Join(dir, base),
		TSRaw:    ts,
		When:     ts,
		Slug:     slug,
		OrigName: featurePrefix + slug,
		Phase:    "-",
		Status:   "-",
	}
	if t, err := time.Parse(tsLayout, ts); err == nil {
		e.When = t.Format("2006-01-02 15:04:05")
	}
	if phase, status := readState(filepath.Join(e.Dir, "state.md")); phase != "" || status != "" {
		if phase != "" {
			e.Phase = phase
		}
		if status != "" {
			e.Status = status
		}
	}
	return e
}

// parseBase splits "<compact-ts>-<slug>" on the first '-' (the compact timestamp
// carries none). A same-second collision suffix ".N" (see uniqueDest) is stripped
// from the slug so a re-delete always restores under the EXACT original name — the
// disambiguating counter never leaks into the parsed slug (REV-011). A name
// without a '-' yields the whole string as the ts.
func parseBase(base string) (ts, slug string) {
	i := strings.IndexByte(base, '-')
	if i < 0 {
		return base, ""
	}
	return base[:i], stripCollisionSuffix(base[i+1:])
}

// stripCollisionSuffix removes a trailing ".N" (N all digits) that uniqueDest
// appends to disambiguate a same-second re-delete, leaving the original slug. A
// dot with a non-numeric tail is an ordinary slug character and is left intact.
func stripCollisionSuffix(slug string) string {
	i := strings.LastIndexByte(slug, '.')
	if i <= 0 || i == len(slug)-1 {
		return slug
	}
	for _, r := range slug[i+1:] {
		if r < '0' || r > '9' {
			return slug
		}
	}
	return slug[:i]
}

var stateLine = regexp.MustCompile(`^\s*-\s*\*\*([^:*]+):\*\*\s*(.*)$`)

// readState pulls just phase/status from a trashed state.md (self-contained
// lenient parse — no coupling to the contract reader). Missing file → empties.
func readState(path string) (phase, status string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		m := stateLine.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(m[1]))
		val := strings.TrimSpace(m[2])
		if i := strings.Index(val, "<!--"); i >= 0 {
			val = strings.TrimSpace(val[:i])
		}
		val = firstToken(val)
		switch key {
		case "phase":
			phase = val
		case "status":
			status = val
		}
	}
	return phase, status
}

func firstToken(s string) string {
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i]
	}
	return s
}

// uniqueDest disambiguates a same-second re-delete of an identically-named slug
// by appending ".2", ".3", … to the trash folder. The counter uses a '.' (never a
// '-') so parseBase's first-'-' split still recovers the exact original slug and
// the restored feature name is never polluted by the counter (REV-011).
func uniqueDest(dest string) string {
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		return dest
	}
	for i := 2; i < 1000; i++ {
		cand := fmt.Sprintf("%s.%d", dest, i)
		if _, err := os.Stat(cand); os.IsNotExist(err) {
			return cand
		}
	}
	return dest
}

// moveDir renames src→dst, falling back to a recursive copy + source removal
// when the two are on different filesystems (os.Rename returns EXDEV). The
// source is only removed AFTER a successful copy — a delete is never destructive
// before the recoverable copy lands.
func moveDir(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	} else if !isCrossDevice(err) {
		return err
	}
	if err := copyTree(src, dst); err != nil {
		_ = os.RemoveAll(dst) // clean a partial copy before surfacing the error
		return err
	}
	return os.RemoveAll(src)
}

// isCrossDevice reports whether a rename failed only because src and dst are on
// different filesystems (the one case a copy fallback is valid for).
func isCrossDevice(err error) bool {
	var le *os.LinkError
	if errors.As(err, &le) {
		return errors.Is(le.Err, syscall.EXDEV)
	}
	return errors.Is(err, syscall.EXDEV)
}

// copyTree recursively copies a directory tree, preserving file modes and
// symlinks. Used only by the cross-device moveDir fallback. It uses os.Lstat (not
// os.Stat) so a symlink is NEVER followed: a link is recreated as a link
// (os.Readlink + os.Symlink), keeping the copy byte-faithful AND making a cyclic
// link (one pointing at an ancestor) safe — we copy the link itself and never
// recurse through it, so no unbounded recursion / stack overflow (REV-009).
func copyTree(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	}
	if info.IsDir() {
		if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyTree(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	return copyFile(src, dst, info.Mode().Perm())
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
