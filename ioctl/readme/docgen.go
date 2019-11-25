package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd"
)

var ioctlPath = filepath.Join(os.Getenv("GOPATH"), "src", "github.com", "iotexproject", "iotex-core", "ioctl")

func main() {
	preString := `# ioctl
ioctl is a command-line interface for interacting with IoTeX blockchains.

# Build
` + "`./buildcli.sh`\n" + `

If you want to build ioctl on Windows, you need to install mingw. Package manager [Chocolatey](https://chocolatey.org/) provides an easy way to intall latest mingw.
` + "`C:\\Windows\\system32> choco install mingw`\n" + `

After this command, target bin files will be placed in ./release/ folder, upload them to
specific release so install-cli.sh can download them.

# Install
## Install released build
    curl --silent https://raw.githubusercontent.com/iotexproject/iotex-core/master/install-cli.sh | sh

## Install latest build
    curl https://raw.githubusercontent.com/iotexproject/iotex-core/master/install-cli.sh | sh -s "unstable"
`

	linkHandler := func(c *cobra.Command, s string) string {
		if c == cmd.RootCmd {
			return "readme/" + s
		}
		if strings.Contains(s, "ioctl.md") {
			return "../README.md"
		}
		return s
	}

	filePrepender := func(s string) string {
		if strings.Contains(s, "README.md") {
			return preString
		}
		return ""
	}

	path := os.Getenv("GOPATH") + "/src/github.com/iotexproject/iotex-core/ioctl/readme"
	err := GenMarkdownTreeCustom(cmd.RootCmd, path, filePrepender, linkHandler)
	if err != nil {
		log.Fatal(err)
	}
}

// GenMarkdownTreeCustom is the the same as GenMarkdownTree, but
// with custom filePrepender and linkHandler.
func GenMarkdownTreeCustom(cmd *cobra.Command, dir string, filePrepender func(string) string,
	linkHandler func(*cobra.Command, string) string) error {
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}
		if err := GenMarkdownTreeCustom(c, dir, filePrepender, linkHandler); err != nil {
			return err
		}
	}

	basename := strings.Replace(cmd.CommandPath(), " ", "_", -1) + ".md"
	filename := filepath.Join(dir, basename)
	if strings.Contains(filename, "ioctl.md") {
		filename = filepath.Join(ioctlPath, "README.md")
	}

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.WriteString(f, filePrepender(filename)); err != nil {
		return err
	}
	if err := GenMarkdownCustom(cmd, f, linkHandler); err != nil {
		return err
	}
	return nil
}

// GenMarkdownCustom creates custom markdown output.
func GenMarkdownCustom(cmd *cobra.Command, w io.Writer, linkHandler func(*cobra.Command, string) string) error {
	cmd.InitDefaultHelpCmd()
	cmd.InitDefaultHelpFlag()

	buf := new(bytes.Buffer)
	name := cmd.CommandPath()

	short := cmd.Short
	long := cmd.Long
	if len(long) == 0 {
		long = short
	}

	buf.WriteString("## " + name + "\n\n")
	buf.WriteString(short + "\n\n")
	buf.WriteString("### Synopsis\n\n")
	buf.WriteString(long + "\n\n")

	if cmd.Runnable() {
		buf.WriteString(fmt.Sprintf("```\n%s\n```\n\n", cmd.UseLine()))
	}

	if len(cmd.Example) > 0 {
		buf.WriteString("### Examples\n\n")
		buf.WriteString(fmt.Sprintf("```\n%s\n```\n\n", cmd.Example))
	}

	if err := printOptions(buf, cmd, name); err != nil {
		return err
	}
	if hasSeeAlso(cmd) {
		buf.WriteString("### SEE ALSO\n\n")
		if cmd.HasParent() {
			parent := cmd.Parent()
			pname := parent.CommandPath()
			link := pname + ".md"
			link = strings.Replace(link, " ", "_", -1)
			buf.WriteString(fmt.Sprintf("* [%s](%s)\t - %s\n", pname, linkHandler(cmd, link), parent.Short))
			cmd.VisitParents(func(c *cobra.Command) {
				if c.DisableAutoGenTag {
					cmd.DisableAutoGenTag = c.DisableAutoGenTag
				}
			})
		}

		children := cmd.Commands()
		sort.Sort(byName(children))

		for _, child := range children {
			if !child.IsAvailableCommand() || child.IsAdditionalHelpTopicCommand() {
				continue
			}
			cname := name + " " + child.Name()
			link := cname + ".md"
			link = strings.Replace(link, " ", "_", -1)
			buf.WriteString(fmt.Sprintf("* [%s](%s)\t - %s\n", cname, linkHandler(cmd, link), child.Short))
		}
		buf.WriteString("\n")
	}
	if !cmd.DisableAutoGenTag {
		buf.WriteString("###### Auto generated by ioctl on " + time.Now().Format("2-Jan-2006") + "\n")
	}
	_, err := buf.WriteTo(w)
	return err
}

func printOptions(buf *bytes.Buffer, cmd *cobra.Command, name string) error {
	flags := cmd.NonInheritedFlags()
	flags.SetOutput(buf)
	if flags.HasAvailableFlags() {
		buf.WriteString("### Options\n\n```\n")
		flags.PrintDefaults()
		buf.WriteString("```\n\n")
	}

	parentFlags := cmd.InheritedFlags()
	parentFlags.SetOutput(buf)
	if parentFlags.HasAvailableFlags() {
		buf.WriteString("### Options inherited from parent commands\n\n```\n")
		parentFlags.PrintDefaults()
		buf.WriteString("```\n\n")
	}
	return nil
}

// Test to see if we have a reason to print See Also information in docs
// Basically this is a test for a parent commend or a subcommand which is
// both not deprecated and not the autogenerated help command.
func hasSeeAlso(cmd *cobra.Command) bool {
	if cmd.HasParent() {
		return true
	}
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}
		return true
	}
	return false
}

type byName []*cobra.Command

func (s byName) Len() int           { return len(s) }
func (s byName) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s byName) Less(i, j int) bool { return s[i].Name() < s[j].Name() }
