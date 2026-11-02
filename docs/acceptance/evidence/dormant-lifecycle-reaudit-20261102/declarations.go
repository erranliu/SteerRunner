package main

import (
 "bytes"
 "encoding/json"
 "fmt"
 "go/ast"
 "go/format"
 "go/parser"
 "go/token"
 "os"
 "os/exec"
 "path/filepath"
 "sort"
 "strings"
)

func git(args ...string) []byte { v,e:=exec.Command("git",args...).Output();if e!=nil{panic(e)};return v }
func declarations(name string, raw []byte) map[string]string {
 out:=map[string]string{};set:=token.NewFileSet();f,e:=parser.ParseFile(set,name,raw,0);if e!=nil{panic(e)}
 emit:=func(key string,n ast.Node){var b bytes.Buffer;if e:=format.Node(&b,set,n);e!=nil{panic(e)};out[key]=b.String()}
 for _,d:=range f.Decls {switch n:=d.(type){
 case *ast.FuncDecl: k:=n.Name.Name;if n.Recv!=nil {var b bytes.Buffer;format.Node(&b,set,n.Recv.List[0].Type);k=b.String()+"."+k};emit("func "+k,n)
 case *ast.GenDecl: for _,s:=range n.Specs {switch v:=s.(type){case *ast.TypeSpec:emit("type "+v.Name.Name,v);case *ast.ValueSpec:for _,id:=range v.Names{emit(n.Tok.String()+" "+id.Name,v)}}}
 }};return out
}
func main(){
 changed:=strings.Fields(string(git("diff","--name-only","HEAD","--","internal","cmd")))
 deleted:=map[string][]string{};modified:=map[string][]string{};identical:=0;retainedTypes:=0;production:=[]string{}
 for _,p:=range changed {if !strings.HasSuffix(p,".go")||strings.HasSuffix(p,"_test.go"){continue};before:=declarations(p,git("show","HEAD:"+p));after:=map[string]string{};raw,e:=os.ReadFile(p);if e==nil{after=declarations(p,raw)}else if !os.IsNotExist(e){panic(e)}
 production=append(production,p)
 for k,v:=range before {next,ok:=after[k];if !ok {deleted[p]=append(deleted[p],k)}else if next!=v{modified[p]=append(modified[p],k)}else{identical++;if strings.HasPrefix(k,"type "){retainedTypes++}}};for k:=range after{if _,ok:=before[k];!ok{panic("unexpected added declaration: "+p+":"+k)}}
 sort.Strings(deleted[p]);sort.Strings(modified[p])
 }
 refs:=map[string][]string{};symbols:=map[string]bool{}
 for _,keys:=range deleted{for _,key:=range keys{if strings.Contains(key,"."){continue};symbols[strings.Fields(key)[1]]=true}}
 files:=strings.Split(strings.TrimSpace(string(git("ls-files","--","*.go"))),"\n")
 for _,p:=range files{p=strings.TrimSpace(p);if strings.HasPrefix(p,"docs/"){continue};raw:=git("show","HEAD:"+p);set:=token.NewFileSet();f,e:=parser.ParseFile(set,p,raw,0);if e!=nil{panic(e)};seen:=map[string]bool{};ast.Inspect(f,func(n ast.Node)bool{if id,ok:=n.(*ast.Ident);ok&&symbols[id.Name]{seen[id.Name]=true};return true});for k:=range seen{refs[k]=append(refs[k],p)}}
 result:=map[string]any{"base_commit":strings.TrimSpace(string(git("rev-parse","HEAD"))),"changed_go_files":changed,"production_files":production,"deleted_declarations":deleted,"modified_declarations":modified,"identical_retained_declarations":identical,"identical_retained_types":retainedTypes,"baseline_identifier_files_excluding_historical_docs":refs,"note":"Identifier inventory includes definitions and same-name identifiers in unrelated packages; reviewed against package and legal entry points. Unchanged files are identical in git diff; comments/imports are excluded from AST comparison."}
 raw,e:=json.MarshalIndent(result,"","  ");if e!=nil{panic(e)};out:=filepath.Join(".evidence","dormant-lifecycle-reaudit","declarations.json");if e=os.WriteFile(out,raw,0600);e!=nil{panic(e)};fmt.Printf("retained declarations=%d retained types=%d output=%s\n",identical,retainedTypes,out)
}
