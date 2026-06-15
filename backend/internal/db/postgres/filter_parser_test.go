package postgres

import (
	"testing"

	"github.com/google/uuid"
)

func TestParseFilterReview(t *testing.T) {
	t.Run("r=1 needs review", func(t *testing.T) {
		sql, n, args, err := ParseFilter("{r=1}", 1)
		if err != nil {
			t.Fatalf("ParseFilter: %v", err)
		}
		if sql != "f.needs_review = $1" {
			t.Fatalf("sql = %q", sql)
		}
		if n != 2 || len(args) != 1 || args[0] != true {
			t.Fatalf("n=%d args=%v", n, args)
		}
	})

	t.Run("r=0 reviewed", func(t *testing.T) {
		sql, _, args, err := ParseFilter("{r=0}", 1)
		if err != nil {
			t.Fatalf("ParseFilter: %v", err)
		}
		if sql != "f.needs_review = $1" || len(args) != 1 || args[0] != false {
			t.Fatalf("sql=%q args=%v", sql, args)
		}
	})

	t.Run("combined with mime", func(t *testing.T) {
		sql, n, args, err := ParseFilter("{r=1,&,m~image/%}", 1)
		if err != nil {
			t.Fatalf("ParseFilter: %v", err)
		}
		if sql != "(f.needs_review = $1 AND mt.name LIKE $2)" {
			t.Fatalf("sql = %q", sql)
		}
		if n != 3 || len(args) != 2 || args[0] != true || args[1] != "image/%" {
			t.Fatalf("n=%d args=%v", n, args)
		}
	})

	t.Run("invalid flag rejected", func(t *testing.T) {
		if _, _, _, err := ParseFilter("{r=2}", 1); err == nil {
			t.Fatal("expected error for r=2")
		}
	})
}

func TestFilterTagUses(t *testing.T) {
	a := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	b := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	tests := []struct {
		name string
		dsl  string
		want map[uuid.UUID]bool // tag → included; absence means "not recorded"
	}{
		{"single included", "{t=" + a.String() + "}", map[uuid.UUID]bool{a: true}},
		{"single excluded", "{!,t=" + a.String() + "}", map[uuid.UUID]bool{a: false}},
		{"double negation is included", "{!,!,t=" + a.String() + "}", map[uuid.UUID]bool{a: true}},
		{
			"and of two included",
			"{t=" + a.String() + ",&,t=" + b.String() + "}",
			map[uuid.UUID]bool{a: true, b: true},
		},
		{
			"not over a group excludes both",
			"{!,(,t=" + a.String() + ",|,t=" + b.String() + ",)}",
			map[uuid.UUID]bool{a: false, b: false},
		},
		{"untagged pseudo-token skipped", "{t=" + uuid.Nil.String() + "}", map[uuid.UUID]bool{}},
		{"mime-only filter records nothing", "{m=3}", map[uuid.UUID]bool{}},
		{"empty filter", "{}", map[uuid.UUID]bool{}},
		{"unparseable filter is best-effort nil", "{t=not-a-uuid}", map[uuid.UUID]bool{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := make(map[uuid.UUID]bool)
			for _, u := range filterTagUses(tc.dsl) {
				got[u.tagID] = u.included
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d uses %v, want %d %v", len(got), got, len(tc.want), tc.want)
			}
			for id, inc := range tc.want {
				if g, ok := got[id]; !ok || g != inc {
					t.Errorf("tag %s: got (included=%v, present=%v), want included=%v", id, g, ok, inc)
				}
			}
		})
	}
}
