package app

func newMockModelForTest() model {
	return newModel(
		generateMockCommits(),
		false,
		"",
		"Mock Mode (test fixture). Changes will be simulated.",
	)
}
