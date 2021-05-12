package rate

import (
	"testing"
	"time"
)

func TestOptions(t *testing.T) {

	rate := New(time.Second, WithBurst(10), WithCleanCutoff(time.Second*3), WithCleanInterval(time.Second*3), WithBucketName("test"))
	if rate.burst != 10 {
		t.Errorf("burst = %d; want 10", rate.burst)
	}
	if rate.cleanCutoff != time.Second*3 {
		t.Errorf("cutoff = %s; want 3s", rate.cleanCutoff)
	}
	if rate.cleanInterval != time.Second*3 {
		t.Errorf("interval = %s; want 3s", rate.cleanInterval)
	}
	if rate.bucketName != "test" {
		t.Errorf("bucketName = %s; want test", rate.bucketName)
	}
}
