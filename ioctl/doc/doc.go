package doc

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// GenMarkdownTreeCustom is the the same as GenMarkdownTree, but
// with custom filePrepender and linkHandler.
func GenMarkdownTreeCustom(c *cobra.Command, dir string, name string, path string, filePrepender func(string) string,
	linkHandler func(*cobra.Command, string) string) (err error) {
	for _, child := range c.Commands() {
		if !child.IsAvailableCommand() || child.IsAdditionalHelpTopicCommand() {
			continue
		}
		if err = GenMarkdownTreeCustom(child, dir, name, path, filePrepender, linkHandler); err != nil {
			return err
		}
	}

	basename := strings.Replace(c.CommandPath(), " ", "_", -1) + ".md"
	filename := filepath.Join(dir, basename)
	if strings.Contains(filename, name+".md") {
		filename = filepath.Join(path, "README.md")
	}

	var f *os.File
	f, err = os.Create(filepath.Clean(filename))
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			return
		}
		err = f.Close()
	}()
	if _, err = io.WriteString(f, filePrepender(filename)); err != nil {
		return err
	}
	if err = GenMarkdownCustom(c, f, linkHandler); err != nil {
		return err
	}
	return err
}

// GenMarkdownCustom creates custom markdown output.
func GenMarkdownCustom(c *cobra.Command, w io.Writer, linkHandler func(*cobra.Command, string) string) error {
	c.InitDefaultHelpCmd()
	c.InitDefaultHelpFlag()

	buf := new(bytes.Buffer)
	name := c.CommandPath()

	short := c.Short
	long := c.Long
	if long == "" {
		long = short
	}

	buf.WriteString("## " + name + "\n\n")
	buf.WriteString(short + "\n\n")
	buf.WriteString("### Synopsis\n\n")
	buf.WriteString(long + "\n\n")

	if c.Runnable() {
		buf.WriteString(fmt.Sprintf("```\n%s\n```\n\n", c.UseLine()))
	}

	if len(c.Example) > 0 {
		buf.WriteString("### Examples\n\n")
		buf.WriteString(fmt.Sprintf("```\n%s\n```\n\n", c.Example))
	}

	if err := printOptions(buf, c); err != nil {
		return err
	}
	if hasSeeAlso(c) {
		buf.WriteString("### SEE ALSO\n\n")
		if c.HasParent() {
			parent := c.Parent()
			pName := parent.CommandPath()
			link := pName + ".md"
			link = strings.Replace(link, " ", "_", -1)
			buf.WriteString(fmt.Sprintf("* [%s](%s)\t - %s\n", pName, linkHandler(c, link), parent.Short))
			c.VisitParents(func(c *cobra.Command) {
				if c.DisableAutoGenTag {
					c.DisableAutoGenTag = c.DisableAutoGenTag
				}
			})
		}

		children := c.Commands()
		sort.Sort(byName(children))

		for _, child := range children {
			if !child.IsAvailableCommand() || child.IsAdditionalHelpTopicCommand() {
				continue
			}
			cname := name + " " + child.Name()
			link := cname + ".md"
			link = strings.Replace(link, " ", "_", -1)
			buf.WriteString(fmt.Sprintf("* [%s](%s)\t - %s\n", cname, linkHandler(c, link), child.Short))
		}
		buf.WriteString("\n")
	}
	if !c.DisableAutoGenTag {
		buf.WriteString("###### Auto generated by docgen on " + time.Now().Format("2-Jan-2006") + "\n")
	}
	_, err := buf.WriteTo(w)
	return err
}

func printOptions(buf *bytes.Buffer, c *cobra.Command) error {
	flags := c.NonInheritedFlags()
	flags.SetOutput(buf)
	if flags.HasAvailableFlags() {
		buf.WriteString("### Options\n\n```\n")
		flags.PrintDefaults()
		buf.WriteString("```\n\n")
	}

	parentFlags := c.InheritedFlags()
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
func hasSeeAlso(c *cobra.Command) bool {
	if c.HasParent() {
		return true
	}
	for _, c := range c.Commands() {
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
