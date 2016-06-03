package commands

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"

	//ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore"
	//bs "github.com/ipfs/go-ipfs/blocks/blockstore"
	k "github.com/ipfs/go-ipfs/blocks/key"
	cmds "github.com/ipfs/go-ipfs/commands"
	cli "github.com/ipfs/go-ipfs/commands/cli"
	files "github.com/ipfs/go-ipfs/commands/files"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/filestore"
	fsutil "github.com/ipfs/go-ipfs/filestore/util"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
)

var FileStoreCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with filestore objects",
	},
	Subcommands: map[string]*cmds.Command{
		"add":      addFileStore,
		"ls":       lsFileStore,
		"ls-files": lsFiles,
		"verify":   verifyFileStore,
		"rm":       rmFilestoreObjs,
		"clean":    cleanFileStore,
		"fix-pins": repairPins,
		"unpinned": fsUnpinned,
		"rm-dups":  rmDups,
		"upgrade":  fsUpgrade,
		"mv":       moveIntoFilestore,
	},
}

var addFileStore = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Add files to the filestore.",
		ShortDescription: `
Add contents of <path> to the filestore.  Most of the options are the
same as for "ipfs add".
`},
	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, true, "The path to a file to be added."),
	},
	Options: addFileStoreOpts(),
	PreRun: func(req cmds.Request) error {
		serverSide,_,_ := req.Option("server-side").Bool()
		if !serverSide {
			err := getFiles(req)
			if err != nil {
				return err
			}
		}
		return AddCmd.PreRun(req)
	},
	Run: func(req cmds.Request, res cmds.Response) {
		config,_ := req.InvocContext().GetConfig()
		serverSide,_,_ := req.Option("server-side").Bool()
		if serverSide && !config.Filestore.APIServerSidePaths {
		 	res.SetError(errors.New("Server Side Adds not enabled."), cmds.ErrNormal)
			return
		}
		if serverSide {
			err := getFiles(req)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
		}
		req.Values()["no-copy"] = true
		AddCmd.Run(req, res)
	},
	PostRun: AddCmd.PostRun,
	Type:    AddCmd.Type,
}

func addFileStoreOpts() []cmds.Option {
	var opts []cmds.Option
	opts = append(opts, AddCmd.Options...)
	opts = append(opts,
		cmds.BoolOption("server-side", "S", "Read file on server."),
	)
	return opts
}

func getFiles(req cmds.Request) error {
	inputs := req.Arguments()
	for _, fn := range inputs {
		if !path.IsAbs(fn) {
			return errors.New("File path must be absolute.")
		}
	}
	_, fileArgs, err := cli.ParseArgs(req, inputs, nil, AddCmd.Arguments, nil)
	if err != nil {
		return err
	}
	file := files.NewSliceFile("", "", fileArgs)
	req.SetFiles(file)
	return nil
}

var lsFileStore = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List objects in filestore",
		ShortDescription: `
List objects in the filestore.  If one or more <obj> is specified only
list those specific objects, otherwise list all objects.  An <obj> can
either be a multihash, or an absolute path.  If the path ends in '/'
than it is assumed to be a directory and all paths with that directory
are included.

If --all is specified list all matching blocks are lists, otherwise
only blocks representing the a file root is listed.  A file root is any
block that represents a complete file.

If --quiet is specified only the hashes are printed, otherwise the
fields are as follows:
  <hash> <type> <filepath> <offset> <size> [<modtime>]
where <type> is one of"
  leaf: to indicate a node where the contents are stored
        to in the file itself
  root: to indicate a root node that represents the whole file
  other: some other kind of node that represent part of a file
  invld: a leaf node that has been found invalid
and <filepath> is the part of the file the object represents.  The
part represented starts at <offset> and continues for <size> bytes.
If <offset> is the special value "-" indicates a file root.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("obj", false, true, "Hash or filename to list."),
	},
	Options: []cmds.Option{
		cmds.BoolOption("quiet", "q", "Write just hashes of objects."),
		cmds.BoolOption("all", "a", "List everything, not just file roots."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		_, fs, err := extractFilestore(req)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		quiet, _, err := req.Option("quiet").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		all, _, err := req.Option("all").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		objs := req.Arguments()
		keys := make([]k.Key, 0)
		paths := make([]string, 0)
		for _, obj := range objs {
			if filepath.IsAbs(obj) {
				paths = append(paths, obj)
			} else {
				keys = append(keys, k.B58KeyDecode(obj))
			}
		}
		if len(keys) > 0 && len(paths) > 0 {
			res.SetError(errors.New("Cannot specify both hashes and paths."), cmds.ErrNormal)
			return
		}

		var ch <-chan fsutil.ListRes
		if len(keys) > 0 {
			ch, _ = fsutil.ListByKey(fs, keys)
		} else if all && len(paths) == 0 && quiet {
			ch, _ = fsutil.ListKeys(fs)
		} else if all && len(paths) == 0 {
			ch, _ = fsutil.ListAll(fs)
		} else if !all && len(paths) == 0 {
			ch, _ = fsutil.ListWholeFile(fs)
		} else if all {
			ch, _ = fsutil.List(fs, func(r fsutil.ListRes) bool {
				return pathMatch(paths, r.FilePath)
			})
		} else {
			ch, _ = fsutil.List(fs, func(r fsutil.ListRes) bool {
				return r.WholeFile() && pathMatch(paths, r.FilePath)
			})
		}

		if quiet {
			res.SetOutput(&chanWriter{ch: ch, quiet: true})
		} else {
			res.SetOutput(&chanWriter{ch: ch})
		}
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

func pathMatch(match_list []string, path string) bool {
	for _, to_match := range match_list {
		if to_match[len(to_match)-1] == filepath.Separator {
			if strings.HasPrefix(path, to_match) {
				return true
			}
		} else {
			if to_match == path {
				return true
			}
		}
	}
	return false

}

var lsFiles = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List files in filestore",
		ShortDescription: `
Lis files in the filestore.  If --quiet is specified only the
file names are printed, otherwise the fields are as follows:
  <filepath> <hash> <size>
`,
	},
	Options: []cmds.Option{
		cmds.BoolOption("quiet", "q", "Write just filenames."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		_, fs, err := extractFilestore(req)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		quiet, _, err := req.Option("quiet").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		ch, _ := fsutil.ListWholeFile(fs)
		res.SetOutput(&chanWriterByFile{ch, "", 0, quiet})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

type chanWriter struct {
	ch     <-chan fsutil.ListRes
	buf    string
	offset int
	errors bool
	quiet  bool
}

func (w *chanWriter) Read(p []byte) (int, error) {
	if w.offset >= len(w.buf) {
		w.offset = 0
		res, more := <-w.ch
		if !more && !w.errors {
			return 0, io.EOF
		} else if !more && w.errors {
			return 0, errors.New("Some checks failed.")
		} else if fsutil.AnError(res.Status) {
			w.errors = true
		}
		if w.quiet {
			w.buf = fmt.Sprintf("%s\n", res.MHash())
		} else {
			w.buf = res.Format()
		}
	}
	sz := copy(p, w.buf[w.offset:])
	w.offset += sz
	return sz, nil
}

type chanWriterByFile struct {
	ch     <-chan fsutil.ListRes
	buf    string
	offset int
	quiet  bool
}

func (w *chanWriterByFile) Read(p []byte) (int, error) {
	if w.offset >= len(w.buf) {
		w.offset = 0
		res, more := <-w.ch
		if !more {
			return 0, io.EOF
		}
		if w.quiet {
			w.buf = fmt.Sprintf("%s\n", res.FilePath)
		} else {
			w.buf = fmt.Sprintf("%s %s %d\n", res.FilePath, res.MHash(), res.Size)
		}
	}
	sz := copy(p, w.buf[w.offset:])
	w.offset += sz
	return sz, nil
}

var verifyFileStore = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Verify objects in filestore",
		ShortDescription: `
Verify <hash> nodes in the filestore.  If no hashes are specified then
verify everything in the filestore.

The output is:
  <status> [<type> <filepath> <offset> <size> [<modtime>]]
where <type>, <filepath>, <offset>, <size> and <modtime> are the same
as in the "ls" command and <status> is one of

  ok:       the original data can be reconstructed
  complete: all the blocks in the tree exists but no attempt was
            made to reconstruct the original data

  incomplete: some of the blocks of the tree could not be read

  changed: the contents of the backing file have changed
  no-file: the backing file could not be found
  error:   the backing file was found but could not be read

  ERROR:   the block could not be read due to an internal error

  found:   the child of another node was found outside the filestore
  missing: the child of another node does not exist
  <blank>: the child of another node node exists but no attempt was
           made to verify it

  appended: the node is still valid but the original file was appended

  orphan: the node is a child of another node that was not found in
          the filestore
 
If --basic is specified then just scan leaf nodes to verify that they
are still valid.  Otherwise attempt to reconstruct the contents of
all nodes and check for orphan nodes if applicable.

The --level option specifies how thorough the checks should be.  A
current meaning of the levels are:
  7-9: always check the contents
  2-6: check the contents if the modification time differs
  0-1: only check for the existence of blocks without verifying the
       contents of leaf nodes

The --verbose option specifies what to output.  The current values are:
  7-9: show everything
  5-6: don't show child nodes unless there is a problem
  3-4: don't show child nodes
  0-2: don't show root nodes unless there is a problem
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("hash", false, true, "Hashs of nodes to verify."),
	},
	Options: []cmds.Option{
		cmds.BoolOption("basic", "Perform a basic scan of leaf nodes only."),
		cmds.IntOption("level", "l", "0-9, Verification level.").Default(6),
		cmds.IntOption("verbose", "v", "0-9 Verbose level.").Default(6),
		cmds.BoolOption("skip-orphans", "Skip check for orphans."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, fs, err := extractFilestore(req)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		args := req.Arguments()
		keys := make([]k.Key, 0)
		for _, key := range args {
			keys = append(keys, k.B58KeyDecode(key))
		}
		basic, _, err := req.Option("basic").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		level, _, err := req.Option("level").Int()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		verbose, _, err := req.Option("verbose").Int()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		if level < 0 || level > 9 {
			res.SetError(errors.New("level must be between 0-9"), cmds.ErrNormal)
			return
		}
		skipOrphans, _, err := req.Option("skip-orphans").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if basic && len(keys) == 0 {
			ch, _ := fsutil.VerifyBasic(fs, level, verbose)
			res.SetOutput(&chanWriter{ch: ch})
		} else if basic {
			ch, _ := fsutil.VerifyKeys(keys, node, fs, level)
			res.SetOutput(&chanWriter{ch: ch})
		} else if len(keys) == 0 {
			ch, _ := fsutil.VerifyFull(node, fs, level, verbose, skipOrphans)
			res.SetOutput(&chanWriter{ch: ch})
		} else {
			ch, _ := fsutil.VerifyKeysFull(keys, node, fs, level, verbose)
			res.SetOutput(&chanWriter{ch: ch})
		}
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

var cleanFileStore = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remove invalid or orphan nodes from the filestore.",
		ShortDescription: `
Removes invalid or orphan nodes from the filestore as specified by
<what>.  <what> is the status of a node as reported by "verify", it
can be any of "changed", "no-file", "error", "incomplete",
"orphan", "invalid" or "full".  "invalid" is an alias for "changed"
and "no-file" and "full" is an alias for "invalid" "incomplete" and
"orphan" (basically remove everything but "error").

It does the removal in three passes.  If there is nothing specified to
be removed in a pass that pass is skipped.  The first pass does a
"verify --basic" and is used to remove any "changed", "no-file" or
"error" nodes.  The second pass does a "verify --level 0
--skip-orphans" and will is used to remove any "incomplete" nodes due
to missing children (the "--level 0" only checks for the existence of
leaf nodes, but does not try to read the content).  The final pass
will do a "verify --level 0" and is used to remove any "orphan" nodes.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("what", true, true, "any of: changed no-file error incomplete orphan invalid full").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption("quiet", "q", "Produce less output."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, fs, err := extractFilestore(req)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		quiet, _, err := req.Option("quiet").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		//_ = node
		//ch, err := fsutil.List(fs, quiet)
		rdr, err := fsutil.Clean(req, node, fs, quiet, req.Arguments()...)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		res.SetOutput(rdr)
		//res.SetOutput(&chanWriter{ch, "", 0, false})
		return
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

var rmFilestoreObjs = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remove objects from the filestore",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("hash", true, true, "Multi-hashes to remove."),
	},
	Options: []cmds.Option{
		cmds.BoolOption("quiet", "q", "Produce less output."),
		cmds.BoolOption("force", "Do Not Abort in non-fatal erros."),
		cmds.BoolOption("direct", "Delete individual blocks."),
		cmds.BoolOption("ignore-pins", "Ignore pins"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, fs, err := extractFilestore(req)
		_ = fs
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		opts := fsutil.DeleteOpts{}
		quiet, _, err := req.Option("quiet").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		opts.Force, _, err = req.Option("force").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		opts.Direct, _, err = req.Option("direct").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		opts.IgnorePins, _, err = req.Option("ignore-pins").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		hashes := req.Arguments()
		rdr, wtr := io.Pipe()
		var rmWtr io.Writer = wtr
		if quiet {
			rmWtr = ioutil.Discard
		}
		go func() {
			keys := make([]k.Key, len(hashes))
			for i, mhash := range hashes {
				keys[i] = k.B58KeyDecode(mhash)
			}
			err = fsutil.Delete(req, rmWtr, node, fs, opts, keys...)
			if err != nil {
				wtr.CloseWithError(err)
				return
			}
			wtr.Close()
		}()
		res.SetOutput(rdr)
		return
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

func extractFilestore(req cmds.Request) (*core.IpfsNode, *filestore.Datastore, error) {
	node, err := req.InvocContext().GetNode()
	if err != nil {
		return nil, nil, err
	}
	fs, ok := node.Repo.SubDatastore(fsrepo.RepoFilestore).(*filestore.Datastore)
	if !ok {
		err := errors.New("Could not extract filestore")
		return nil, nil, err
	}
	return node, fs, nil
}

var repairPins = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Repair pins to non-existent or incomplete objects",
	},
	Options: []cmds.Option{
		cmds.BoolOption("dry-run", "n", "Report on what will be done."),
		cmds.BoolOption("skip-root", "Don't repin root in broken recursive pin."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, fs, err := extractFilestore(req)
		if err != nil {
			return
		}
		dryRun, _, err := req.Option("dry-run").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		skipRoot, _, err := req.Option("skip-root").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		r, w := io.Pipe()
		go func() {
			defer w.Close()
			fsutil.RepairPins(node, fs, w, dryRun, skipRoot)
		}()
		res.SetOutput(r)
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

var fsUnpinned = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List unpinned whole-file objects in filestore.",
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, fs, err := extractFilestore(req)
		if err != nil {
			return
		}
		r, w := io.Pipe()
		go func() {
			err := fsutil.Unpinned(node, fs, w)
			if err != nil {
				w.CloseWithError(err)
			} else {
				w.Close()
			}
		}()
		res.SetOutput(r)
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

var rmDups = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remove duplicate blocks stored outside filestore.",
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, fs, err := extractFilestore(req)
		if err != nil {
			return
		}
		r, w := io.Pipe()
		go func() {
			err := fsutil.RmDups(w, fs, node.Blockstore)
			if err != nil {
				w.CloseWithError(err)
			} else {
				w.Close()
			}
		}()
		res.SetOutput(r)
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

var fsUpgrade = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Upgrade filestore to most recent format.",
	},
	Run: func(req cmds.Request, res cmds.Response) {
		_, fs, err := extractFilestore(req)
		if err != nil {
			return
		}
		r, w := io.Pipe()
		go func() {
			err := fsutil.Upgrade(w, fs)
			if err != nil {
				w.CloseWithError(err)
			} else {
				w.Close()
			}
		}()
		res.SetOutput(r)
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

var moveIntoFilestore = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Move a Node representing file into the filestore.",
		ShortDescription: `
Move a node representing a file into the filestore.  For now the old
copy is not removed.  Use "filestore rm-dups" to remove the old copy.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("hash", true, false, "Multi-hash to move."),
		cmds.StringArg("file", false, false, "File to store node's content in."),
	},
	Options: []cmds.Option{},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		offline := !node.OnlineMode()
		args := req.Arguments()
		if len(args) < 1 {
			res.SetError(errors.New("Must specify hash."), cmds.ErrNormal)
			return
		}
		if len(args) > 2 {
			res.SetError(errors.New("Too many arguments."), cmds.ErrNormal)
			return
		}
		mhash := args[0]
		key := k.B58KeyDecode(mhash)
		path := ""
		if len(args) == 2 {
			path = args[1]
		} else {
			path = mhash
		}
		if offline {
			path, err = filepath.Abs(path)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
		}
		rdr, wtr := io.Pipe()
		go func() {
			err := fsutil.ConvertToFile(node, key, path)
			if err != nil {
				wtr.CloseWithError(err)
				return
			}
			wtr.Close()
		}()
		res.SetOutput(rdr)
		return
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}
