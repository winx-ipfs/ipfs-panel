package node

import "os"

func osEnviron() []string { return os.Environ() }
