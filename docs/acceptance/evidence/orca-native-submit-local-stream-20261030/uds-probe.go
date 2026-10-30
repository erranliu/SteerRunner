package main

import (
 "encoding/json"
 "fmt"
 "net"
 "os"
 "path/filepath"
)

func main() {
 if len(os.Args) == 3 && os.Args[1] == "inspect" {
  dir := os.Args[2]
  record := map[string]any{"directory":dir}
  root, err := os.OpenRoot(dir)
  record["open_root_error"] = fmt.Sprint(err)
  if err == nil { info,e:=root.Lstat("s"); record["root_lstat_error"]=fmt.Sprint(e); if info!=nil {record["root_lstat_mode"]=info.Mode().String()}; root.Close() }
  info,e:=os.Lstat(filepath.Join(dir,"s")); record["absolute_lstat_error"]=fmt.Sprint(e); if info!=nil {record["absolute_lstat_mode"]=info.Mode().String()}
  json.NewEncoder(os.Stdout).Encode(record)
  return
 }
 if len(os.Args) == 3 && os.Args[1] == "base" { os.Setenv("TMP",os.Args[2]); os.Setenv("TEMP",os.Args[2]) }
 dir, err := os.MkdirTemp("", "sr-uds-")
 if err != nil { panic(err) }
 record := map[string]any{"directory": dir}
 root, err := os.OpenRoot(dir)
 if err != nil { panic(err) }
 socket := filepath.Join(dir,"s")
 listener, err := net.Listen("unix", socket)
 record["listen_error"] = fmt.Sprint(err)
 if err == nil {
  listener.(*net.UnixListener).SetUnlinkOnClose(false)
  info, e := root.Lstat("s")
  record["root_lstat_error"] = fmt.Sprint(e)
  if info != nil { record["root_lstat_mode"] = info.Mode().String() }
  absolute, e := os.Lstat(socket)
  record["absolute_lstat_error"] = fmt.Sprint(e)
  if absolute != nil { record["absolute_lstat_mode"] = absolute.Mode().String() }
  record["listener_close_error"] = fmt.Sprint(listener.Close())
  record["socket_remove_error"] = fmt.Sprint(root.Remove("s"))
 }
 record["root_close_error"] = fmt.Sprint(root.Close())
 record["directory_remove_error"] = fmt.Sprint(os.Remove(dir))
 json.NewEncoder(os.Stdout).Encode(record)
}
