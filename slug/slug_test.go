package slug

import "testing"

func TestSlug(t *testing.T) {
	type slugTest struct {
		ID       int
		Text     string
		Expected string
	}
	examples := []slugTest{
		{ID: 10, Text: "This is a Sentence", Expected: "10-this-is-a-sentence"},
		{ID: 99, Text: "OTHER things like a $", Expected: "99-other-things-like-a"},
		{ID: 87, Text: "multiple      spaces", Expected: "87-multiple-spaces"},
	}

	for _, example := range examples {
		actual := Slug(example.ID, example.Text)
		if actual != example.Expected {
			t.Errorf("Expected:\n%v\nGot:\n%v\n", example.Expected, actual)
		}
	}
}

func TestParse(t *testing.T) {
	type Example struct {
		Slug     string
		Expected int
	}
	examples := []Example{
		{Slug: "342-hello-there", Expected: 342},
		{Slug: "99288-umm-yeah", Expected: 99288},
	}

	for _, example := range examples {
		actual, _ := Parse(example.Slug)
		if actual != example.Expected {
			t.Errorf("Expected:\n%v\nGot:\n%v\n", example.Expected, actual)
		}
	}
}
