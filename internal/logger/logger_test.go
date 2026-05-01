func TestUserColour(t *testing.T) {
    // Capture output
    var buf bytes.Buffer
    out := os.Stdout
    os.Stdout = &buf
    defer func() { os.Stdout = out }()

    logger.User("test")
    output := buf.String()
    expected := "\x1b[32mtest\x1b[0m\n" // depends on default light colours
    assert.Contains(t, output, "\x1b[32m")
}
