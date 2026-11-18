package agent

import (
	"encoding/json"
	"fmt"
 "io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BuildHistoryRecord is a private, device-local snapshot of a completed build.
type BuildHistoryRecord struct {
	ID          string             `json:"id"`
	SiteID      string             `json:"site_id"`
	OutputBytes int                `json:"output_bytes"`
	Result      AndroidBuildResult `json:"result"`
}

const buildHistoryRetention = 10

func buildHistoryDirectory(dataRoot, siteID string) (string, error) {
	if !validSiteID(siteID) {
		return "", fmt.Errorf("invalid site id")
	}
	if err := os.MkdirAll(dataRoot, 0o700); err != nil {
		return "", fmt.Errorf("prepare Agent data directory: %w", err)
	}
	root, err := ensureSafeSubdirectory(dataRoot, "build-history", 0o700)
	if err != nil {
		return "", fmt.Errorf("prepare build history directory: %w", err)
	}
	return ensureSafeSubdirectory(root, siteID, 0o700)
}

func persistBuildHistory(dataRoot, siteID string, result AndroidBuildResult) error {
	dir, err := buildHistoryDirectory(dataRoot, siteID)
	if err != nil {
		return err
	}
	finished := result.FinishedAt
	if finished.IsZero() {
		finished = time.Now().UTC()
	}
	id := finished.UTC().Format("20060102T150405.000000000Z")
	record := BuildHistoryRecord{ID: id, SiteID: siteID, OutputBytes: len(result.Output), Result: result}
	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode build history: %w", err)
	}
	target := filepath.Join(dir, id+".json")
	temp := filepath.Join(dir, "."+id+".tmp")
	file, err := os.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create build history record: %w", err)
	}
	if _, err = file.Write(encoded); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("write build history record: %w", err)
	}
	if closeErr != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("close build history record: %w", closeErr)
	}
	if err := os.Rename(temp, target); err != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("finalize build history record: %w", err)
	}
	return pruneBuildHistory(dataRoot,siteID,dir,buildHistoryRetention)
}

func pruneBuildHistory(dataRoot,siteID,dir string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var names []string
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	if len(names) <= keep {
		return nil
	}
	for _, name := range names[:len(names)-keep] {
  path:=filepath.Join(dir,name)
  if raw,readErr:=os.ReadFile(path);readErr==nil{
   var record BuildHistoryRecord
   if json.Unmarshal(raw,&record)==nil&&record.SiteID==siteID&&record.Result.ArtifactDir!=""{
    if cleanupErr:=removeBuildArtifactDirectory(dataRoot,siteID,record.Result.ArtifactDir);cleanupErr!=nil{return fmt.Errorf("prune artifacts for build %s: %w",record.ID,cleanupErr)}
   }
  }
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("prune build history %s: %w", name, err)
		}
	}
	return nil
}

// ListBuildHistory returns newest-first local records for one site.
func ListBuildHistory(dataRoot, siteID string, limit int) ([]BuildHistoryRecord, error) {
	dir, err := buildHistoryDirectory(dataRoot, siteID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > buildHistoryRetention {
		limit = buildHistoryRetention
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, entry.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	if len(names) > limit {
		names = names[:limit]
	}
	records := make([]BuildHistoryRecord, 0, len(names))
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("read build history %s: %w", name, err)
		}
		var record BuildHistoryRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("decode build history %s: %w", name, err)
		}
		if record.SiteID != siteID {
			return nil, fmt.Errorf("build history record %s has mismatched site id", name)
		}
		record.OutputBytes = len(record.Result.Output)
		record.Result.Output = ""
		records = append(records, record)
	}
	return records, nil
}

// GetBuildHistory returns one full record, including its stored log output.
func GetBuildHistory(dataRoot, siteID, id string) (BuildHistoryRecord, error) {
	if !validSiteID(siteID) {
		return BuildHistoryRecord{}, fmt.Errorf("invalid site id")
	}
	if _, err := time.Parse("20060102T150405.000000000Z", id); err != nil {
		return BuildHistoryRecord{}, fmt.Errorf("invalid build history id")
	}
	dir, err := buildHistoryDirectory(dataRoot, siteID)
	if err != nil {
		return BuildHistoryRecord{}, err
	}
	path := filepath.Join(dir, id+".json")
	info, err := os.Lstat(path)
	if err != nil {
		return BuildHistoryRecord{}, fmt.Errorf("build history record not found: %w", err)
	}
	if !info.Mode().IsRegular() {
		return BuildHistoryRecord{}, fmt.Errorf("build history record is not a regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return BuildHistoryRecord{}, err
	}
	var record BuildHistoryRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return BuildHistoryRecord{}, fmt.Errorf("decode build history record: %w", err)
	}
	if record.SiteID != siteID || record.ID != id {
		return BuildHistoryRecord{}, fmt.Errorf("build history record identity mismatch")
	}
	record.OutputBytes = len(record.Result.Output)
	return record, nil
}

type BuildArtifactFile struct {
 Path string `json:"path"`
 SizeBytes int64 `json:"size_bytes"`
 ModifiedAt time.Time `json:"modified_at"`
}
func buildArtifactRoot(dataRoot,siteID,buildID string)(string,error){
 record,err:=GetBuildHistory(dataRoot,siteID,buildID);if err!=nil{return "",err}
 if !record.Result.Success{return "",fmt.Errorf("artifacts are available only for successful builds")}
 resolvedExpected,err:=safeExistingSubdirectory(dataRoot,filepath.Join("artifacts",siteID));if err!=nil{return "",fmt.Errorf("artifact directory not found: %w",err)}
 resolved,err:=filepath.EvalSymlinks(record.Result.ArtifactDir);if err!=nil{return "",fmt.Errorf("artifact directory not found")}
 rel,err:=filepath.Rel(resolvedExpected,resolved);if err!=nil||rel==".."||strings.HasPrefix(rel,".."+string(filepath.Separator)){return "",fmt.Errorf("artifact directory escapes site storage")}
 info,err:=os.Stat(resolved);if err!=nil||!info.IsDir(){return "",fmt.Errorf("artifact directory is invalid")}
 return resolved,nil
}
func ListBuildArtifacts(dataRoot,siteID,buildID string)([]BuildArtifactFile,error){
 root,err:=buildArtifactRoot(dataRoot,siteID,buildID);if err!=nil{return nil,err}
 files:=make([]BuildArtifactFile,0)
 err=filepath.WalkDir(root,func(path string,d os.DirEntry,walkErr error)error{
  if walkErr!=nil{return walkErr};if path==root||d.IsDir(){return nil};if d.Type()&os.ModeSymlink!=0||!d.Type().IsRegular(){return nil}
  info,e:=d.Info();if e!=nil{return e};rel,e:=filepath.Rel(root,path);if e!=nil{return e}
  files=append(files,BuildArtifactFile{Path:filepath.ToSlash(rel),SizeBytes:info.Size(),ModifiedAt:info.ModTime().UTC()})
  if len(files)>2000{return fmt.Errorf("artifact file count exceeds 2000")}
  return nil
 })
 if err!=nil{return nil,fmt.Errorf("list build artifacts: %w",err)}
 sort.Slice(files,func(i,j int)bool{return files[i].Path<files[j].Path});return files,nil
}
func ReadBuildArtifactChunk(dataRoot,siteID,buildID,relativePath string,offset int64,limit int)([]byte,int64,bool,error){
 if relativePath==""||filepath.IsAbs(relativePath)||strings.Contains(relativePath,"\\")||offset<0{return nil,0,false,fmt.Errorf("invalid artifact path or offset")}
 clean:=filepath.Clean(filepath.FromSlash(relativePath));if clean=="."||clean==".."||strings.HasPrefix(clean,".."+string(filepath.Separator)){return nil,0,false,fmt.Errorf("artifact path escapes directory")}
 if limit<=0||limit>256*1024{limit=256*1024}
 root,err:=buildArtifactRoot(dataRoot,siteID,buildID);if err!=nil{return nil,0,false,err}
 path:=filepath.Join(root,clean);info,err:=os.Lstat(path);if err!=nil{return nil,0,false,fmt.Errorf("artifact not found: %w",err)}
 if !info.Mode().IsRegular()||info.Mode()&os.ModeSymlink!=0{return nil,0,false,fmt.Errorf("artifact must be a regular file")}
 resolved,err:=filepath.EvalSymlinks(path);if err!=nil{return nil,0,false,err};rel,err:=filepath.Rel(root,resolved);if err!=nil||rel==".."||strings.HasPrefix(rel,".."+string(filepath.Separator)){return nil,0,false,fmt.Errorf("artifact resolves outside its build directory")}
 if offset>info.Size(){return nil,info.Size(),false,fmt.Errorf("artifact offset exceeds file size")}
 file,err:=os.Open(resolved);if err!=nil{return nil,0,false,err};defer file.Close()
 buf:=make([]byte,limit);n,readErr:=file.ReadAt(buf,offset);if readErr!=nil&&readErr!=io.EOF{return nil,info.Size(),false,readErr}
 return buf[:n],info.Size(),offset+int64(n)>=info.Size(),nil
}

func removeBuildArtifactDirectory(dataRoot,siteID,path string)error{
 root,err:=safeExistingSubdirectory(dataRoot,filepath.Join("artifacts",siteID));if os.IsNotExist(err){return nil};if err!=nil{return err}
 resolved,err:=filepath.EvalSymlinks(path);if err!=nil{if os.IsNotExist(err){return nil};return err}
 rel,err:=filepath.Rel(root,resolved);if err!=nil||rel=="."||rel==".."||strings.HasPrefix(rel,".."+string(filepath.Separator)){return fmt.Errorf("refusing to remove artifact path outside site storage")}
 info,err:=os.Lstat(path);if err!=nil{if os.IsNotExist(err){return nil};return err};if info.Mode()&os.ModeSymlink!=0||!info.IsDir(){return fmt.Errorf("artifact path is not a real directory")}
 return os.RemoveAll(path)
}
