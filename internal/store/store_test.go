package store

import (
	"path/filepath"
	"testing"
)

func TestStoreUpsertAndQuery(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	cat := Categorize("archive.zip")
	if cat != "Compressed" {
		t.Fatalf("expected Compressed, got %s", cat)
	}

	task := TaskRecord{
		ID:           "test-1",
		URL:          "http://example.com/file.zip",
		Filename:     "file.zip",
		FilePath:     "C:/downloads/file.zip",
		TotalBytes:   1000,
		Downloaded:   500,
		Status:       "downloading",
		Connections:  4,
		Category:     cat,
		AcceptRanges: true,
	}

	if err := db.UpsertTask(task); err != nil {
		t.Fatal(err)
	}

	fetched, err := db.GetTask("test-1")
	if err != nil {
		t.Fatal(err)
	}
	if fetched.Downloaded != 500 || fetched.Category != "Compressed" {
		t.Fatalf("unexpected task record: %+v", fetched)
	}

	chunks := []ChunkRecord{
		{TaskID: "test-1", Idx: 0, StartOffset: 0, EndOffset: 499, Downloaded: 500, Status: "done"},
		{TaskID: "test-1", Idx: 1, StartOffset: 500, EndOffset: 999, Downloaded: 0, Status: "pending"},
	}
	if err := db.SaveChunks("test-1", chunks); err != nil {
		t.Fatal(err)
	}

	chs, err := db.GetChunks("test-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(chs) != 2 || chs[0].Downloaded != 500 {
		t.Fatalf("unexpected chunks: %+v", chs)
	}
}
