package main

func main() {
	/*

		1. Collect inventory
		2. Construct Job
			1. Collect tasks
			2. Collect assets
			3. Collect vars
		3. Iterate over inventory, for each:
			1. Ensure chief-whip-local present
				1. Run local bash script
				2. Upload chief-whip-local
			2. SSH to target, serialize Job on its stdin
			3. Read status reports (1 json obj per task)

	*/
}
