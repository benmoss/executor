/*
Executor component tests require garden-linux to be running, and can be found
in github.com/cloudfoundry-incubator/inigo
*/
package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestExecutor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Executor Integration Suite")
}

var executorPath string

var _ = SynchronizedBeforeSuite(func() []byte {
	executorPath, err := gexec.Build("github.com/cloudfoundry-incubator/executor/cmd/executor", "-race")
	Ω(err).ShouldNot(HaveOccurred())
	return []byte(executorPath)
}, func(path []byte) {
	executorPath = string(path)
})

var _ = SynchronizedAfterSuite(func() {
	//noop
}, func() {
	gexec.CleanupBuildArtifacts()
})
