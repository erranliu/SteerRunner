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
 b "github.com/erranliu/SteerRunner/internal/execution/backend"
 "github.com/erranliu/SteerRunner/internal/resources"
 "github.com/erranliu/SteerRunner/internal/runstate"
)
func main() {
 if len(os.Args)!=2 { panic("output file required") }
 types:=map[string]string{}
 wanted:=map[string]bool{"LifecycleSnapshot":true,"LifecycleInput":true,"LifecycleRequest":true,"LifecycleReceipt":true,"Operation":true,"LifecycleOperationIdentity":true,"TurnInput":true,"Record":true,"HostActionInput":true,"HostActionAuthorization":true,"ProbeIdentity":true,"PreparationFacts":true}
 for _,name:=range []string{"internal/execution/backend/lifecycle.go","internal/execution/backend/operation.go","internal/execution/backend/session.go","internal/execution/backend/host.go","internal/runstate/record.go"} {
  set:=token.NewFileSet(); f,e:=parser.ParseFile(set,name,nil,0); if e!=nil {panic(e)}
  for _,d:=range f.Decls {g,ok:=d.(*ast.GenDecl);if !ok {continue}; for _,s:=range g.Specs {v,ok:=s.(*ast.TypeSpec);if !ok||!wanted[v.Name.Name] {continue}; var buf bytes.Buffer;if e:=format.Node(&buf,set,v);e!=nil {panic(e)};types[v.Name.Name]=buf.String()} }
 }
 snap:=b.LifecycleSnapshot{PendingDeliveryID:"saved-delivery",PreparationProbe:&b.ProbeIdentity{Execution:"execution",Nonce:"nonce"},HostAuthorization:&b.HostActionAuthorization{ExecutionID:"execution",BlockerVersion:3},OriginalHost:&b.ProcessIdentity{Kind:"os",PID:7},Operations:[]b.Operation{{Version:1,ID:"operation",Action:string(b.LifecycleCleanup),Request:[]byte(`{"resources":[]}`)}},PendingTurn:&b.TurnInput{CommandID:"command",Prompt:"original"}}
 values:=map[string]any{"snapshot":snap,"request":b.LifecycleRequest{Snapshot:snap,Input:b.LifecycleInput{Host:&b.HostActionInput{Kind:b.HostOpenOriginal}}},"receipt":b.LifecycleReceipt{OriginalHost:snap.OriginalHost},"record":runstate.Record{ID:"run",Snapshot:snap},"capabilities":resources.Capabilities(),"resource_digest":resources.Digest(),"persisted_types":types}
 raw,e:=json.Marshal(values);if e!=nil {panic(e)};if e=os.WriteFile(os.Args[1],raw,0600);e!=nil {panic(e)}
 fmt.Println("captured immutable wire/types/capabilities baseline")
}
