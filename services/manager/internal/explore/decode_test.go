package explore

import (
	"strings"
	"testing"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/pins"
)

func TestInterpretShapes(t *testing.T) {
	pb := pins.CIDInfo{Version: 1, Codec: 0x70, Hash: 0x12}
	folder, err := interpret(pb, []byte(`{"Data":{"/":{"bytes":"CAE"}},"Links":[{"Hash":{"/":"bafkqac3imvwgy3zao5xxe3de"},"Name":"notes.txt","Tsize":21},{"Hash":{"/":"bafkqac3imvwgy3zao5xxe3de"},"Name":"","Tsize":0}]}`), "")
	if err != nil {
		t.Fatal(err)
	}
	if folder.Kind != "folder" || folder.Codec != "dag-pb" || folder.Hash != "sha2-256" || folder.Size != nil || len(folder.Links) != 2 {
		t.Fatalf("folder %+v", folder)
	}
	if folder.Links[0].Name != "notes.txt" || folder.Links[0].Size == nil || *folder.Links[0].Size != 21 {
		t.Fatalf("link %+v", folder.Links[0])
	}
	if folder.Links[1].Name != "" || folder.Links[1].Size == nil || *folder.Links[1].Size != 0 {
		t.Fatalf("unnamed %+v", folder.Links[1])
	}

	file, err := interpret(pb, []byte(`{"Data":{"/":{"bytes":"CAISBWhlbGxvGAU"}},"Links":[]}`), "")
	if err != nil {
		t.Fatal(err)
	}
	if file.Kind != "file" || file.Text != "hello" || file.Size == nil || *file.Size != 5 {
		t.Fatalf("file %+v", file)
	}

	link, err := interpret(pb, []byte(`{"Data":{"/":{"bytes":"CAQSBC90bXA"}},"Links":[]}`), "rest")
	if err != nil {
		t.Fatal(err)
	}
	if link.Kind != "symlink" || link.Text != "/tmp" || link.Focus != "rest" {
		t.Fatalf("symlink %+v", link)
	}

	hamt, err := interpret(pb, []byte(`{"Data":{"/":{"bytes":"CAU"}},"Links":[]}`), "")
	if err != nil || hamt.Kind != "folder" {
		t.Fatalf("hamt %+v %v", hamt, err)
	}

	rawInfo := pins.CIDInfo{Version: 1, Codec: 0x55, Hash: 0x00}
	raw, err := interpret(rawInfo, []byte(`{"/":{"bytes":"aGVsbG8gd29ybGQ"}}`), "")
	if err != nil {
		t.Fatal(err)
	}
	if raw.Kind != "bytes" || raw.Text != "hello world" || raw.Hash != "identity" || raw.Size == nil || *raw.Size != 11 {
		t.Fatalf("raw %+v", raw)
	}

	dataInfo := pins.CIDInfo{Version: 1, Codec: 0x71, Hash: 0x12}
	data, err := interpret(dataInfo, []byte(`{"hello":"world","photo":{"/":"bafkqac3imvwgy3zao5xxe3de"}}`), "")
	if err != nil {
		t.Fatal(err)
	}
	if data.Kind != "data" || len(data.Links) != 1 || data.Links[0].Name != "photo" || data.Links[0].CID != "bafkqac3imvwgy3zao5xxe3de" {
		t.Fatalf("data %+v", data)
	}
	if !strings.Contains(string(data.Data), `"hello":"world"`) {
		t.Fatalf("json %s", data.Data)
	}

	binary, err := interpret(pb, []byte(`{"Data":{"/":{"bytes":"/w=="}},"Links":[]}`), "")
	if err != nil {
		t.Fatal(err)
	}
	// 0xff is not a UnixFS type field, so the block stays unnamed.
	if binary.Kind != "block" || binary.Text != "" {
		t.Fatalf("binary %+v", binary)
	}
}

func TestInterpretTruncatesLinks(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"Data":{"/":{"bytes":"CAE"}},"Links":[`)
	for i := 0; i < maxLinks+1; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`{"Hash":{"/":"bafkqac3imvwgy3zao5xxe3de"},"Name":"n","Tsize":1}`)
	}
	b.WriteString(`]}`)
	block, err := interpret(pins.CIDInfo{Version: 1, Codec: 0x70, Hash: 0x12}, []byte(b.String()), "")
	if err != nil {
		t.Fatal(err)
	}
	if !block.LinksTruncated || len(block.Links) != maxLinks {
		t.Fatalf("truncated %d %v", len(block.Links), block.LinksTruncated)
	}
}

func TestInterpretOmitsHugeData(t *testing.T) {
	body := `{"blob":"` + strings.Repeat("a", maxData) + `"}`
	block, err := interpret(pins.CIDInfo{Version: 1, Codec: 0x71, Hash: 0x12}, []byte(body), "")
	if err != nil {
		t.Fatal(err)
	}
	if !block.DataOmitted || len(block.Data) != 0 || block.Kind != "data" {
		t.Fatalf("omitted %+v", block)
	}
}
