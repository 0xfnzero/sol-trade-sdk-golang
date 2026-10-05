package perf

import (
	"bytes"
	"os"
	"testing"
	"unsafe"
)

func TestReviewPointerAlignment(t *testing.T) {
	storage := make([]byte, 1024)
	for offset := 0; offset < 512; offset++ {
		p := unsafe.Pointer(&storage[offset])
		aligned := Align(p, 512)
		if !IsAligned(aligned, 512) {
			t.Fatal("unaligned pointer")
		}
		distance := uintptr(aligned) - uintptr(p)
		if distance >= 512 {
			t.Fatal("alignment escaped allocation")
		}
		*(*byte)(aligned) = 42
		if storage[offset+int(distance)] != 42 {
			t.Fatal("wrong backing allocation")
		}
	}
}

func TestReviewAlignedReadRetainsBacking(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "aligned")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	data := bytes.Repeat([]byte{42}, 1024)
	if _, err = file.Write(data); err != nil {
		t.Fatal(err)
	}
	reader := DirectIOFile{fd: int(file.Fd()), directIO: true}
	got, err := reader.Read(100, 17)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data[17:117]) {
		t.Fatal("aligned read returned wrong bytes")
	}
}
