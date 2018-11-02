# Concourse Go Style Guide

This document serves to collect some guiding principles and resources to
consider when writing Go code for Concourse.

## Idiomatic Go

Generally the preferred manner of writing Go code follows the style and
conventions present in the [Go project's Standard Library
Packages](https://golang.org/pkg/#stdlib).  The style present in the stdlib
packages is often referred to as "Idiomatic Go", and serves as a consistent set
of conventions for Go programmers to follow in their projects.

In lieu of inventing Concourse's own set of styling rules for Go, below are some
useful resources which capture the sentiment of "Idiomatic Go".

### [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
> This page collects common comments made during reviews of Go code, so that a
> single detailed explanation can be referred to by shorthands. This is a 
> laundry list of common mistakes, not a comprehensive style guide.

### [Idiomatic Go](https://dmitri.shuralyov.com/idiomatic-go)
A good resource covering some common styling and pattern issues which deviate
from the idiomatic Go style and best practice. Some great demonstrations of the
importance of consistency and clarity; and where subtle differences matter
greatly

### [Go Proverbs](https://go-proverbs.github.io/)
A list of simple proverbs to consider when writing Go, with links to
[@rob_pike](https://twitter.com/rob_pike)'s talk of the same name.

Some *especially important* ones to keep in mind for Concourse:
- [Gofmt's style is no one's favorite, yet gofmt is everyone's favorite.](https://www.youtube.com/watch?v=PAAkCSZUG1c&t=8m43s)
- [Clear is better than clever.](https://www.youtube.com/watch?v=PAAkCSZUG1c&t=14m35s)
- [Don't just check errors, handle them gracefully.](https://www.youtube.com/watch?v=PAAkCSZUG1c&t=17m25s)
- [Documentation is for users.](https://www.youtube.com/watch?v=PAAkCSZUG1c&t=19m07s)

## Logging

[Lager](https://github.com/cloudfoundry/lager) is our preferred logging framework for its opinionated structure, JSON
format, and punny name. Lager's log output is best when all of its features are properly leveraged by the component
doing the logging. Here are some things to consider when logging:

### Log Actions Not Messages
Each of [Lager's emission functions](https://github.com/cloudfoundry/lager/blob/1a7fb7de44c33d258236e250189099e48a4c7430/logger.go#L16-L19)'
signature takes a string named "action" as the first argument. It is generally
preferable to treat this value as the (terse) name of the action being
performed in proximity to the log - with words separated by `-` -  rather than
a (more verbose) message.

> eg. `lager.Info("writing-styleguide")` rather than `lager.Info("writing the style guide now")`


### Provide Valuable Data When Emitting Logs
Every new session, and each emission function can have a `lager.Data` provided, which allows arbitrary 
key-value data to be added to each log from the session, or individual log actions respectively.

Be sure to provide any relevant data (especially at the ERROR and DEBUG levels) which might help someone/something
reading those logs discern what happened.

> eg. 
> ```
> logger.Info("writing-styleguide", lager.Data{
> 		"section" : "Logging",
> })
> ```


## Testing

### Unit Test Frameworks

We use [Ginkgo](https://github.com/onsi/ginkgo) to write expressive BDD tests and 
[Gomega](https://github.com/onsi/gomega) for its 
[rich matching library](http://onsi.github.io/gomega/#provided-matchers) and compatibility with Ginkgo.

## Common Concourse Go Gotchas

The Concourse Go codebase is by no means perfect, or reflective of the
standards above, Its often easy to get tripped up, or follow the same pattern
as existing code, leading to some systemic patterns emerging in the code. Below
are some common ones to look out for to avoid, or address when you come across
them.

### A Public Interface For the Sake of a Mockable Interface

Extensive unit tests are present in most packages in the Concourse Go
code, and often [counterfeiter](https://github.com/maxbrunsfeld/counterfeiter) is
employed to auto-generate mocks for interfaces. Mocking out an interface where
the implementation has side effects is often desirable, but it can occasionally
lead to exported interfaces which serve little utility other than a surface
area to mock out for a test.

Its important to consider what each package's public interface is before going
through the motions of making a public interface and a private struct which
implements that interface immediately after in the same `.go` file.


Generally it is preferable to define smaller interfaces where dependencies are
consumed. Interfaces living with implementation leads to direct coupling in
source code, whereas smaller interfaces defined by the consumer encourages
[LSP](https://en.wikipedia.org/wiki/Liskov_substitution_principle) and
[Dependency Inversion](https://en.wikipedia.org/wiki/Dependency_inversion_principle) See:
[Dave Cheney's Tweet on the Subject](https://twitter.com/davecheney/status/942593128355192832)

### Naming Packages and Their Contents

Naming anything is hard, but finding a concise name for a package which describes
its purpose is important in Go code. The same goes for the contents of packages -
especially the publicly exported contents available to other parts of the code.

The Go Blog has a great article on Package Names, which outlines the importance of
naming. Below are some of those examples with references to Concourse Go Code.

**Avoid stutter.**
> Since client code uses the package name as a prefix when referring to the
> package contents, the names for those contents need not repeat the package
> name. The HTTP server provided by the http package is called Server, not
> HTTPServer. Client code refers to this type as http.Server, so there is no
> ambiguity.

eg. 
[`atc/worker.Worker`](https://github.com/concourse/concourse/blob/b216f374dd1fe7824d271418ce0035c44d50cbf0/atc/worker/worker.go#L30-L49)

**Simplify function names.**
> When a function in package pkg returns a value of type pkg.Pkg (or
> *pkg.Pkg), the function name can often omit the type name without > confusion:

eg.
[`skymarshal/dexserver.NewDexServer`](https://github.com/concourse/concourse/blob/b216f374dd1fe7824d271418ce0035c44d50cbf0/skymarshal/dexserver/dexserver.go#L28)






