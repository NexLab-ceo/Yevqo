package agent

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func writeTestSiteArchive(t *testing.T, path string, headers []*tar.Header, contents []string) {
	t.Helper()
	file,err:=os.Create(path);if err!=nil{t.Fatal(err)}
	gz:=gzip.NewWriter(file);tw:=tar.NewWriter(gz)
	for i,h:=range headers{if err:=tw.WriteHeader(h);err!=nil{t.Fatal(err)};if h.Typeflag==tar.TypeReg{if _,err:=tw.Write([]byte(contents[i]));err!=nil{t.Fatal(err)}}}
	if err:=tw.Close();err!=nil{t.Fatal(err)};if err:=gz.Close();err!=nil{t.Fatal(err)};if err:=file.Close();err!=nil{t.Fatal(err)}
}

func TestExtractSiteArchiveRejectsTraversalAndLinks(t *testing.T) {
	for _,header:=range []*tar.Header{
		{Name:"../../escape",Mode:0600,Size:1,Typeflag:tar.TypeReg},
		{Name:"link",Mode:0777,Typeflag:tar.TypeSymlink,Linkname:"/etc/passwd"},
	} {
		dir:=t.TempDir();archive:=filepath.Join(dir,"backup.tar.gz")
		writeTestSiteArchive(t,archive,[]*tar.Header{header},[]string{"x"})
		if err:=extractSiteArchive(archive,filepath.Join(dir,"out"));err==nil{t.Fatalf("expected archive entry %q to be rejected",header.Name)}
	}
}

func TestExtractSiteArchiveRestoresRegularFiles(t *testing.T) {
	dir:=t.TempDir();archive:=filepath.Join(dir,"backup.tar.gz");out:=filepath.Join(dir,"out")
	writeTestSiteArchive(t,archive,[]*tar.Header{{Name:"public",Mode:0755,Typeflag:tar.TypeDir},{Name:"public/index.html",Mode:0644,Size:5,Typeflag:tar.TypeReg}},[]string{"","hello"})
	if err:=os.Mkdir(out,0700);err!=nil{t.Fatal(err)}
	if err:=extractSiteArchive(archive,out);err!=nil{t.Fatal(err)}
	data,err:=os.ReadFile(filepath.Join(out,"public","index.html"));if err!=nil{t.Fatal(err)}
	if string(data)!="hello"{t.Fatalf("unexpected restored content: %q",data)}
}
