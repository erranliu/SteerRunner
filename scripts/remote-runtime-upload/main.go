// Upload an explicitly selected public binary/archive to an isolated Orca
// staging workspace. Verify its SHA256 on the execution host before installing.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	"os"
	"time"
)

func main() {
	env := flag.String("environment", "", "paired environment")
	runtime := flag.String("runtime", "", "expected runtime")
	worktree := flag.String("worktree", "", "isolated staging workspace")
	source := flag.String("source", "", "public source artifact")
	relative := flag.String("relative", ".sr-runtime-stage/runtime.zip", "staging relative path")
	flag.Parse()
	f, err := os.Open(*source)
	if err == nil {
		defer f.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		err = orca.UploadWorkspaceArtifact(ctx, *env, *runtime, *worktree, *relative, f)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("artifact uploaded; verify execution-host SHA256 before use")
}
