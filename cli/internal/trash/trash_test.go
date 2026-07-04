package trash

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mkFeature creates a .gogo/work/feature-<slug>/ folder with a state.md and one
// extra file, returning the repo root + the feature dir.
func mkFeature(t *testing.T, slug, state string) (root, dir string) {
	t.Helper()
	root = t.TempDir()
	dir = filepath.Join(root, ".gogo", "work", "feature-"+slug)
	if err := os.MkdirAll(filepath.Join(dir, "review"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "state.md"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "review", "issues.json"), []byte(`{"slug":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, dir
}

func TestName(t *testing.T) {
	ts := time.Date(2026, 7, 4, 2, 45, 0, 0, time.UTC)
	if got := Name(ts, "my-slug"); got != "20260704T024500Z-my-slug" {
		t.Errorf("Name = %q", got)
	}
}

func TestParseBase(t *testing.T) {
	// The compact timestamp carries no '-', so the first '-' splits ts from a
	// slug that may itself contain dashes.
	ts, slug := parseBase("20260704T024500Z-analyst-uat-and-cli-ops")
	if ts != "20260704T024500Z" || slug != "analyst-uat-and-cli-ops" {
		t.Errorf("parseBase = %q / %q", ts, slug)
	}
}

func TestMoveToTrashThenRestore(t *testing.T) {
	root, dir := mkFeature(t, "my-slug", "- **phase:** review\n- **status:** reviewing\n")

	entry, err := MoveToTrash(root, dir)
	if err != nil {
		t.Fatalf("MoveToTrash: %v", err)
	}
	// Source is gone; trash entry holds the whole folder (state.md + subtree).
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("source folder still exists after move")
	}
	if _, err := os.Stat(filepath.Join(entry.Dir, "review", "issues.json")); err != nil {
		t.Errorf("nested file did not move into trash: %v", err)
	}
	if entry.Slug != "my-slug" || entry.OrigName != "feature-my-slug" {
		t.Errorf("entry slug/orig = %q / %q", entry.Slug, entry.OrigName)
	}
	// The trashed state.md drives the "was" hint.
	if entry.Phase != "review" || entry.Status != "reviewing" {
		t.Errorf("entry phase/status = %q / %q, want review/reviewing", entry.Phase, entry.Status)
	}

	// List sees exactly one entry.
	list, err := List(root)
	if err != nil || len(list) != 1 || list[0].Base != entry.Base {
		t.Fatalf("List = %+v, err=%v", list, err)
	}

	// Restore puts it back under the original name.
	dest, err := Restore(root, entry.Base)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if filepath.Base(dest) != "feature-my-slug" {
		t.Errorf("restored to %q", dest)
	}
	if _, err := os.Stat(filepath.Join(dest, "review", "issues.json")); err != nil {
		t.Errorf("nested file did not restore: %v", err)
	}
	// Trash entry is consumed by the restore.
	if _, err := os.Stat(entry.Dir); !os.IsNotExist(err) {
		t.Errorf("trash entry still present after restore")
	}
}

func TestRestoreRefusesCollision(t *testing.T) {
	root, dir := mkFeature(t, "dup", "- **phase:** plan\n")
	entry, err := MoveToTrash(root, dir)
	if err != nil {
		t.Fatalf("MoveToTrash: %v", err)
	}
	// Re-create a live feature with the same name — restore must refuse.
	if err := os.MkdirAll(filepath.Join(root, ".gogo", "work", "feature-dup"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Restore(root, entry.Base); err == nil {
		t.Errorf("Restore clobbered an existing folder — must refuse")
	}
	// The trash entry is left intact after a refused restore.
	if _, err := os.Stat(entry.Dir); err != nil {
		t.Errorf("refused restore consumed the trash entry: %v", err)
	}
}

func TestRestoreMissingEntry(t *testing.T) {
	root := t.TempDir()
	if _, err := Restore(root, "20260704T000000Z-nope"); err == nil {
		t.Errorf("Restore of a missing entry should error")
	}
}

func TestListEmpty(t *testing.T) {
	root := t.TempDir() // no .gogo/trash yet
	list, err := List(root)
	if err != nil {
		t.Fatalf("List on absent trash: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("List = %v, want empty", list)
	}
}

// TestCopyTreeFallback exercises the cross-device fallback's core (copy the tree
// preserving structure) directly — EXDEV cannot be forced portably in a unit
// test, but copyTree is the fallback's substance.
func TestCopyTreeFallback(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("A"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("B"), 0o600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(root, "dst")
	if err := copyTree(src, dst); err != nil {
		t.Fatalf("copyTree: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "sub", "b.txt")); string(b) != "B" {
		t.Errorf("nested file not copied")
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "a.txt")); string(b) != "A" {
		t.Errorf("top file not copied")
	}
}

// TestCopyTreeSymlinkNoRecurse pins REV-009: the cross-device fallback copies a
// symlink AS a link (os.Readlink + os.Symlink), never following it — so a cyclic
// link (one pointing at an ancestor) cannot drive unbounded recursion, and the
// copy stays byte-faithful. os.Stat used to report the cyclic link as a dir and
// recurse until the stack overflowed; os.Lstat sees the link and stops.
func TestCopyTreeSymlinkNoRecurse(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "f.txt"), []byte("F"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A CYCLIC symlink: src/sub/loop -> .. (its own parent). Following it recurses
	// forever; copyTree must recreate the link and never descend into it.
	if err := os.Symlink("..", filepath.Join(src, "sub", "loop")); err != nil {
		t.Skipf("symlinks unsupported on this platform: %v", err)
	}
	// A plain relative symlink to a sibling file — must be preserved as a link.
	if err := os.Symlink("f.txt", filepath.Join(src, "sub", "link")); err != nil {
		t.Skipf("symlinks unsupported on this platform: %v", err)
	}

	dst := filepath.Join(root, "dst")
	if err := copyTree(src, dst); err != nil {
		t.Fatalf("copyTree with a cyclic symlink recursed/errored: %v", err)
	}

	// The cyclic link is recreated as a link (never followed → no recursion).
	li, err := os.Lstat(filepath.Join(dst, "sub", "loop"))
	if err != nil {
		t.Fatalf("cyclic link not copied: %v", err)
	}
	if li.Mode()&os.ModeSymlink == 0 {
		t.Errorf("cyclic entry was dereferenced into a real dir, not preserved as a symlink")
	}
	if got, _ := os.Readlink(filepath.Join(dst, "sub", "loop")); got != ".." {
		t.Errorf("cyclic link target = %q, want ..", got)
	}
	// The sibling link is preserved with its original target (byte-faithful).
	if got, err := os.Readlink(filepath.Join(dst, "sub", "link")); err != nil || got != "f.txt" {
		t.Errorf("sibling link = %q (err %v), want f.txt", got, err)
	}
	// The real file still copied faithfully alongside the links.
	if b, _ := os.ReadFile(filepath.Join(dst, "sub", "f.txt")); string(b) != "F" {
		t.Errorf("real file not copied")
	}
}

// TestMoveToTrashRefusesOutsideWork pins REV-010: the destructive API refuses any
// source that does not resolve under <root>/.gogo/work/ — belt-and-suspenders for
// the append-only changelog (D3/FR6), independent of the single TUI column guard.
func TestMoveToTrashRefusesOutsideWork(t *testing.T) {
	root := t.TempDir()
	// A changelog archive dir — must NEVER be trashable.
	cl := filepath.Join(root, ".gogo", "changelog", "2026-07-04-shipped-thing")
	if err := os.MkdirAll(cl, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cl, "report.md"), []byte("# shipped"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := MoveToTrash(root, cl); err == nil {
		t.Fatalf("MoveToTrash accepted a .gogo/changelog path — must refuse")
	}
	// Nothing moved: the changelog dir is untouched and no trash entry was created.
	if _, err := os.Stat(filepath.Join(cl, "report.md")); err != nil {
		t.Errorf("changelog dir was disturbed by a refused trash: %v", err)
	}
	if list, _ := List(root); len(list) != 0 {
		t.Errorf("a refused trash still created an entry: %+v", list)
	}
}

// TestCollisionSuffixParseSafe pins REV-011 at the unit level: uniqueDest's
// same-second collision suffix uses '.' (…-slug.2), so parseBase still recovers
// the EXACT original slug and the entry restores under the true feature name — the
// counter never leaks into the parsed slug.
func TestCollisionSuffixParseSafe(t *testing.T) {
	root := t.TempDir()
	dir := trashDir(root)
	base := "20260704T024500Z-my-slug"
	if err := os.MkdirAll(filepath.Join(dir, base), 0o755); err != nil {
		t.Fatal(err)
	}
	// A colliding destination is disambiguated with ".2" (parse-safe), not "-2".
	if got := filepath.Base(uniqueDest(filepath.Join(dir, base))); got != base+".2" {
		t.Fatalf("uniqueDest = %q, want %q", got, base+".2")
	}
	// The disambiguated base still parses to the ORIGINAL slug (counter stripped).
	if ts, slug := parseBase(base + ".2"); ts != "20260704T024500Z" || slug != "my-slug" {
		t.Errorf("parseBase(%q) = %q / %q, want …/my-slug", base+".2", ts, slug)
	}
	// And the Entry restores under the true original name, not feature-my-slug.2.
	if e := entryFor(dir, base+".2"); e.Slug != "my-slug" || e.OrigName != "feature-my-slug" {
		t.Errorf("entry slug/orig = %q / %q, want my-slug / feature-my-slug", e.Slug, e.OrigName)
	}
	// A legitimate '.' with a non-numeric tail inside a slug is left intact.
	if _, s := parseBase("20260704T024500Z-my.slug"); s != "my.slug" {
		t.Errorf("non-counter dot was stripped: %q", s)
	}
}

// TestSameSecondDoubleTrashRestores pins the REV-011 end-to-end guarantee: two
// deletes of the same slug both carry the TRUE original name, so restoring the
// first recreates feature-my-slug and restoring the second REFUSES on the name
// collision (never a polluted feature-my-slug.2 / -2 folder).
func TestSameSecondDoubleTrashRestores(t *testing.T) {
	root, dir := mkFeature(t, "my-slug", "- **phase:** plan\n")
	e1, err := MoveToTrash(root, dir)
	if err != nil {
		t.Fatalf("first trash: %v", err)
	}
	// Re-create the same slug under the SAME root and trash it again. Back-to-back
	// calls land in one UTC second (the ".2" suffix path); a rare second-boundary
	// yields a fresh ts — both must restore to the same original name.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "state.md"), []byte("- **phase:** plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e2, err := MoveToTrash(root, dir)
	if err != nil {
		t.Fatalf("second trash: %v", err)
	}
	if e1.Base == e2.Base {
		t.Fatalf("both trashes produced the same base %q — collision not disambiguated", e1.Base)
	}
	// Both entries carry the true original name regardless of the collision suffix.
	if e1.OrigName != "feature-my-slug" || e2.OrigName != "feature-my-slug" {
		t.Fatalf("entries not both feature-my-slug: %q / %q", e1.OrigName, e2.OrigName)
	}
	// Restoring the first recreates the EXACT original folder.
	dest, err := Restore(root, e1.Base)
	if err != nil || filepath.Base(dest) != "feature-my-slug" {
		t.Fatalf("restore1 = %q, err %v", dest, err)
	}
	// The second now refuses — feature-my-slug already lives (collision), proving
	// its parsed name is the same original, not a polluted -2 variant.
	if _, err := Restore(root, e2.Base); err == nil {
		t.Errorf("second restore should refuse on the name collision")
	}
}

// TestMoveDirRenamePath verifies the happy os.Rename path leaves no source.
func TestMoveDirRenamePath(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "s")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(root, "d")
	if err := moveDir(src, dst); err != nil {
		t.Fatalf("moveDir: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source survived a same-device move")
	}
	if _, err := os.Stat(filepath.Join(dst, "f")); err != nil {
		t.Errorf("dest missing the moved file")
	}
}
