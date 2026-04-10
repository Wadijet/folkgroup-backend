package decisionlive

import "testing"

func TestFormatLiveStepNarrativeVi_OmitsEmpty(t *testing.T) {
	got := FormatLiveStepNarrativeVi("A", "", "B", "", "C")
	want := LiveStepPrefixPurpose + "A\n" + LiveStepPrefixLogic + "B\n" + LiveStepPrefixNext + "C"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
