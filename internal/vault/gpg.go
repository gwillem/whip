package vault

/*
	// return runCommand("gpg", "-qd", path)

echo hoi | gpg -e -r info@sansec.io -a --trust-model=always

// runCommand executes the specified command with given arguments and returns its stdout as an os.File
// func runCommand(name string, args ...string) (*os.File, error) {
// 	// Create a pipe
// 	r, w, err := os.Pipe()
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Set up the command that will write to the write-end of our pipe
// 	cmd := exec.Command(name, args...)
// 	cmd.Stdout = w
// 	cmd.Stderr = os.Stderr

// 	// Start the command
// 	if err := cmd.Start(); err != nil {
// 		w.Close()
// 		r.Close()
// 		return nil, err
// 	}

// 	// Close the write end of the pipe in the current goroutine after command starts
// 	go func() {
// 		defer w.Close()
// 		cmd.Wait()
// 	}()

// 	// Return the read end of the pipe
// 	return r, nil
// }
//
*/
