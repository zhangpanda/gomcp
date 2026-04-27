// White-box tests: version index [versionLatest] must match [resolveToolVersionScan] invariants.
package gomcp

import (
	"testing"
)

func vHandler(tag string) HandlerFunc {
	return func(ctx *Context) (*CallToolResult, error) {
		return TextResult(tag), nil
	}
}

// TestIndexMatchesScan_UnversionedName verifies the fast path and full scan return the same tool key.
func TestIndexMatchesScan_UnversionedName(t *testing.T) {
	orders := permutations3([]string{"1.0.0", "1.5.0", "2.0.0"})
	for i, order := range orders {
		s := New("deep", "1")
		for _, ver := range order {
			s.Tool("svc", "d", vHandler(ver), Version(ver))
		}
		s.mu.RLock()
		a, oka := s.resolveToolVersion("svc")
		b, okb := s.resolveToolVersionScan("svc")
		s.mu.RUnlock()
		if !oka || !okb {
			t.Fatalf("perm %d order %v: want both ok, oka=%v okb=%v", i, order, oka, okb)
		}
		if a.info.Name != b.info.Name {
			t.Fatalf("perm %d: fast path name %q != scan %q", i, a.info.Name, b.info.Name)
		}
		if a.info.Name != "svc@2.0.0" {
			t.Fatalf("perm %d: want latest svc@2.0.0, got %q", i, a.info.Name)
		}
	}
}

func TestIndexMatchesScan_PrefixInBaseName(t *testing.T) {
	// Base name includes "@"; fast path is skipped, scan must still work and stay consistent.
	s := New("deep", "1")
	s.Tool("a@b", "d", vHandler("x"), Version("1.0"))
	s.Tool("a@b", "d", vHandler("y"), Version("2.0"))
	s.mu.RLock()
	a, oka := s.resolveToolVersion("a@b")
	b, okb := s.resolveToolVersionScan("a@b")
	s.mu.RUnlock()
	if !oka || !okb || a.info.Name != b.info.Name {
		t.Fatalf("got oka=%v okb=%v a=%q b=%q", oka, okb, a.info.Name, b.info.Name)
	}
	if a.info.Name != "a@b@2.0" {
		t.Fatalf("want a@b@2.0, got %q", a.info.Name)
	}
}

func TestIndexNoFalsePositive_WrongVersionQuery(t *testing.T) {
	s := New("deep", "1")
	s.Tool("calc", "d", vHandler("1"), Version("1.0"))
	s.mu.RLock()
	a, oka := s.resolveToolVersion("calc@9.9")
	b, okb := s.resolveToolVersionScan("calc@9.9")
	s.mu.RUnlock()
	if oka || okb {
		t.Fatalf("expected not found, oka=%v okb=%v name=%q", oka, okb, a.info.Name)
	}
	if a.info.Name != b.info.Name { // both zero
		t.Fatalf("empty mismatch")
	}
}

func TestIndex_SingleVersion(t *testing.T) {
	s := New("deep", "1")
	s.Tool("only", "d", vHandler("one"), Version("1.0.0"))
	s.mu.RLock()
	a, ok := s.resolveToolVersion("only")
	b, sc := s.resolveToolVersionScan("only")
	s.mu.RUnlock()
	if !ok || a.info.Name != "only@1.0.0" || a.info.Name != b.info.Name {
		t.Fatalf("got ok=%v name=%q scanName=%q", ok, a.info.Name, b.info.Name)
	}
	_ = sc
}

func TestIndex_ThreeDigitSemver(t *testing.T) {
	s := New("deep", "1")
	// 10.0.0 > 2.0.0 numerically
	s.Tool("x", "d", vHandler("a"), Version("2.0.0"))
	s.Tool("x", "d", vHandler("b"), Version("10.0.0"))
	s.mu.RLock()
	a, _ := s.resolveToolVersion("x")
	b, _ := s.resolveToolVersionScan("x")
	s.mu.RUnlock()
	if a.info.Name != "x@10.0.0" || a.info.Name != b.info.Name {
		t.Fatalf("want x@10.0.0, got a=%q b=%q", a.info.Name, b.info.Name)
	}
}

// TestIndex_StaleKeyFallsBackToScan: if versionLatest points to a removed key, scan still resolves.
func TestIndex_StaleKeyFallsBackToScan(t *testing.T) {
	s := New("deep", "1")
	s.Tool("z", "d", vHandler("1"), Version("1.0"))
	s.Tool("z", "d", vHandler("2"), Version("2.0"))
	s.mu.Lock()
	s.versionLatest["z"] = "z@ghost"
	s.mu.Unlock()
	s.mu.RLock()
	a, oka := s.resolveToolVersion("z")
	b, okb := s.resolveToolVersionScan("z")
	s.mu.RUnlock()
	if !oka || !okb || a.info.Name != b.info.Name {
		t.Fatalf("want fallback scan, oka=%v okb=%v a=%q b=%q", oka, okb, a.info.Name, b.info.Name)
	}
	if a.info.Name != "z@2.0" {
		t.Fatalf("want z@2.0, got %q", a.info.Name)
	}
}

// permutations3 returns all 3! orderings of v.
func permutations3(v []string) [][]string {
	if len(v) != 3 {
		panic("permutations3: need len 3")
	}
	return [][]string{
		{v[0], v[1], v[2]},
		{v[0], v[2], v[1]},
		{v[1], v[0], v[2]},
		{v[1], v[2], v[0]},
		{v[2], v[0], v[1]},
		{v[2], v[1], v[0]},
	}
}
