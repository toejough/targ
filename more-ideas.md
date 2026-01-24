[x] Update the readme, requirements, and architecture to call out a desire to prevent surprises: with declarative
execution control, global flags, and local flags, there's plenty of room for unexpected behavior, which is why we
consistently error any time there's a potential for confusion.
[x] re-examine the requirements, architecture, implementation, and the readme: does the readme accurately reflect and
describe the key use cases, goals, and design decisions made in the implementation? Does the readme lay out the taxonomy
and the features in a way that makes sense to a new user?
[] examine the state of our tests. are they all property-based tests using imptest, rapid, and gomega? are there fuzz
tests for the non-enumerable inputs?
