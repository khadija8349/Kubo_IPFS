package commands

import (
	"fmt"
	"io"

	cmds "github.com/jbenet/go-ipfs/commands"
	"github.com/jbenet/go-ipfs/core"
	"github.com/jbenet/go-ipfs/importer"
	dag "github.com/jbenet/go-ipfs/merkledag"
)

// Error indicating the max depth has been exceded.
var ErrDepthLimitExceeded = fmt.Errorf("depth limit exceeded")

type AddOutput struct {
	Added []Object
}

var addCmd = &cmds.Command{
	Options: []cmds.Option{
		cmds.Option{[]string{"recursive", "r"}, cmds.Bool},
	},
	Arguments: []cmds.Argument{
		cmds.Argument{"file", cmds.ArgFile, false, true},
	},
	Help: "TODO",
	Run: func(res cmds.Response, req cmds.Request) {
		n := req.Context().Node

		// if recursive, set depth to reflect so
		//opt, found := req.Option("r")
		//if r, _ := opt.(bool); found && r {
		//}

		added := make([]Object, len(req.Arguments()))

		// add every path in args
		for i, arg := range req.Arguments() {
			// Add the file
			node, err := add(n, arg.(io.Reader))
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			k, err := node.Key()
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			added[i] = Object{k.String(), nil}
		}

		res.SetOutput(&AddOutput{added})
	},
	Format: func(res cmds.Response) ([]byte, error) {
		v := res.Output().(*AddOutput).Added
		if len(v) == 1 {
			s := fmt.Sprintf("Added object: %s\n", v[0].Hash)
			return []byte(s), nil
		}

		s := fmt.Sprintf("Added %v objects:\n", len(v))
		for _, obj := range v {
			s += fmt.Sprintf("- %s\n", obj.Hash)
		}
		return []byte(s), nil
	},
	Type: &AddOutput{},
}

func add(n *core.IpfsNode, in io.Reader) (*dag.Node, error) {
	node, err := importer.NewDagFromReader(in)
	if err != nil {
		return nil, err
	}

	// add the file to the graph + local storage
	err = n.DAG.AddRecursive(node)
	if err != nil {
		return nil, err
	}

	// ensure we keep it
	return node, n.Pinning.Pin(node, true)
}
