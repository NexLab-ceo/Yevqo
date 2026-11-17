package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"veltrix/internal/shared"
)

func pruneBackups(req shared.PruneBackups) (*shared.PruneBackupsResult, error) {
	if req.Keep < 1 || req.Keep > 50 { return nil, fmt.Errorf("keep must be between 1 and 50") }
	prefix := os.Getenv("PREFIX")
	if prefix == "" { return nil, fmt.Errorf("Termux PREFIX is not set") }
	var dir string
	var match func(string) bool
	switch {
	case req.SiteId != "" && req.ServerID == 0 && validSiteID(req.SiteId):
		dir = filepath.Join(prefix,"var","lib","veltrix-agent","backups","sites",req.SiteId)
		match = func(name string) bool { return strings.HasPrefix(name,req.SiteId+"-") && strings.HasSuffix(name,".tar.gz") }
	case req.SiteId == "" && req.ServerID > 0:
		dir = filepath.Join(prefix,"var","lib","veltrix-agent","backups","databases",fmt.Sprint(req.ServerID))
		match = func(name string) bool { return strings.HasSuffix(name,".dump") }
	default:
		return nil, fmt.Errorf("specify exactly one valid site or database server")
	}
	relative, err := filepath.Rel(prefix, dir)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) { return nil, fmt.Errorf("invalid backup directory") }
	dirReal, err := safeExistingSubdirectory(prefix, relative)
	if os.IsNotExist(err) { return &shared.PruneBackupsResult{Removed:[]string{}},nil }
	if err != nil { return nil, err }
	entries, err := os.ReadDir(dirReal)
	if err != nil { return nil,err }
	type candidate struct { name string; modified int64 }
	files := make([]candidate,0)
	for _,entry := range entries {
		if !match(entry.Name()) || entry.Type()&os.ModeSymlink!=0 { continue }
		info,err:=entry.Info();if err!=nil{return nil,err}
		if !info.Mode().IsRegular(){continue}
		files=append(files,candidate{name:entry.Name(),modified:info.ModTime().UnixNano()})
	}
	sort.Slice(files,func(i,j int)bool{return files[i].modified>files[j].modified})
	removed:=make([]string,0)
	if len(files) > req.Keep {
	for _,file:=range files[req.Keep:] {
		full:=filepath.Join(dirReal,file.name)
		info,err:=os.Lstat(full);if err!=nil{return nil,err}
		if !info.Mode().IsRegular()||info.Mode()&os.ModeSymlink!=0{return nil,fmt.Errorf("refusing to remove non-regular backup file")}
		if err:=os.Remove(full);err!=nil{return nil,err}
		removed=append(removed,file.name)
	}
	}
	return &shared.PruneBackupsResult{Removed:removed},nil
}
