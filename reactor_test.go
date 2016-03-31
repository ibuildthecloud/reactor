package reactor

import (
	"errors"
	"strconv"
	"testing"

	"github.com/Sirupsen/logrus"

	. "gopkg.in/check.v1"
)

var (
	seq int
)

func Test(t *testing.T) { TestingT(t) }

type ReactorSuite struct{}

var _ = Suite(&ReactorSuite{})

func (s *ReactorSuite) SetUpSuite(c *C) {
	logrus.SetLevel(logrus.DebugLevel)
}

type TestNode struct {
	Executed bool
	Id       string
	Err      error
}

func NewTestNode() *TestNode {
	seq++
	return &TestNode{
		Id: strconv.Itoa(seq),
	}
}

func (t *TestNode) Run() error {
	if t.Executed {
		panic("Already executed: " + t.Id)
	}
	t.Executed = true
	return t.Err
}

func (s *ReactorSuite) TestEmpty(c *C) {
	r := NewDefault()
	defer r.Close()

	err := r.ExecuteAndWait()
	c.Assert(err, IsNil)
}

func (s *ReactorSuite) TestOne(c *C) {
	r := NewDefault()
	defer r.Close()

	one := NewTestNode()

	r.Submit(one.Id, one.Run)

	err := r.ExecuteAndWait(one.Id)
	c.Assert(err, IsNil)
	c.Assert(one.Executed, Equals, true)
}

func (s *ReactorSuite) TestSingleDependency(c *C) {
	r := NewDefault()
	defer r.Close()

	one := NewTestNode()
	two := NewTestNode()
	three := NewTestNode()

	r.Submit(one.Id, one.Run)
	r.Submit(two.Id, two.Run, one.Id)
	r.Submit(three.Id, three.Run)

	err := r.ExecuteAndWait(two.Id)
	c.Assert(err, IsNil)
	c.Assert(one.Executed, Equals, true)
	c.Assert(two.Executed, Equals, true)
	c.Assert(three.Executed, Equals, false)
}

func (s *ReactorSuite) TestSingleErr(c *C) {
	r := NewDefault()
	defer r.Close()

	one := NewTestNode()
	one.Err = errors.New("test error")
	two := NewTestNode()
	three := NewTestNode()

	r.Submit(one.Id, one.Run)
	r.Submit(two.Id, two.Run, one.Id)
	r.Submit(three.Id, three.Run)

	err := r.ExecuteAndWait(two.Id)
	c.Assert(err, Equals, one.Err)
	c.Assert(one.Executed, Equals, true)
	c.Assert(two.Executed, Equals, false)
	c.Assert(three.Executed, Equals, false)
}

func (s *ReactorSuite) TestCascadingErr(c *C) {
	r := NewDefault()
	defer r.Close()

	one := NewTestNode()
	one.Err = errors.New("test error")
	two := NewTestNode()
	three := NewTestNode()

	r.Submit(one.Id, one.Run)
	r.Submit(two.Id, two.Run, one.Id)
	r.Submit(three.Id, three.Run, two.Id)

	err := r.ExecuteAndWait(three.Id)
	c.Assert(err, Equals, one.Err)
	c.Assert(one.Executed, Equals, true)
	c.Assert(two.Executed, Equals, false)
	c.Assert(three.Executed, Equals, false)
}

func (s *ReactorSuite) TestMissingDependency(c *C) {
	r := NewDefault()
	defer r.Close()

	one := NewTestNode()

	r.Submit(one.Id, one.Run, "two", "three")

	err := r.ExecuteAndWait(one.Id)
	tErr, ok := err.(ErrMissingDependency)
	c.Assert(ok, Equals, true)
	c.Assert(tErr.Ids, DeepEquals, []string{"two", "three"})
}

func (s *ReactorSuite) TestBlockedButValidDoesntRun(c *C) {
	r := NewDefault()
	defer r.Close()

	node := NewTestNode()
	a1 := NewTestNode()
	a2 := NewTestNode()
	a3 := NewTestNode()
	a3.Err = errors.New("test error")
	b1 := NewTestNode()

	r.Submit(node.Id, node.Run, a1.Id, b1.Id)
	r.Submit(a1.Id, a1.Run, a2.Id)
	r.Submit(a2.Id, a2.Run, a3.Id)
	r.Submit(a3.Id, a3.Run)
	r.Submit(b1.Id, b1.Run)

	err := r.ExecuteAndWait(node.Id)
	c.Assert(err, DeepEquals, a3.Err)
	c.Assert(b1.Executed, Equals, true)
	c.Assert(a1.Executed, Equals, false)
	c.Assert(a3.Executed, Equals, true)
	c.Assert(node.Executed, Equals, false)
}
