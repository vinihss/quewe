package worker

import "testing"

func TestValidateste(t *testing.T) {
	w := Worker{WorkerID: "w1", Concurrency: 2, Status: StatusActive}
	if err := w.Validateste(); err != nil {
		t.Fatal(err)
	}
	cases := []Worker{
		{WorkerID: "", Concurrency: 1, Status: StatusActive},
		{WorkerID: "w", Concurrency: 0, Status: StatusActive},
		{WorkerID: "w", Concurrency: 1, Status: "bogus"},
	}
	for _, c := range cases {
		if err := c.Validateste(); err == nil {
			t.Errorf("expected error for %+v", c)
		}
	}
}
