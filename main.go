package main // Defines the main application package

import ( // Start import block
	"bytes"         // Imports the bytes package for buffer manipulation
	"crypto/tls"    // Imports crypto/tls for configuring SSL/TLS
	"io"            // Imports the io package for I/O primitives
	"log"           // Imports the log package for logging functionalities
	"net/http"      // Imports the net/http package for HTTP client and server
	"net/url"       // Imports the net/url package for URL parsing
	"os"            // Imports the os package for operating system utilities
	"path/filepath" // Imports filepath for path manipulation
	"regexp"        // Imports regexp for regular expressions
	"strings"       // Imports strings for string manipulation
	"time"          // Imports time for time-related functions

	"golang.org/x/net/html" // Imports the external HTML parsing library
) // End import block

// Main entry point for the PDF scraping application.
func main() { // Define the main function
	// URLs to fetch HTML content from to find PDF links.
	sourceURLs := []string{ // Declare and initialize a slice of strings for source URLs
		"https://msdspds.bp.com/msdspds/msdspds.nsf/BPResults?OpenForm&c=All&l=All&p=&n=&b=All&t=All&autosearch=No&autoload=No&sitelang=EN&output=Full&spu=Lubricants&unrestrictedmb=No&cols=0", // The specific URL to scrape
	}
	htmlFilePath := "bp.html"           // Path where the fetched HTML content will be stored.
	outputDirectory := "PDFs"           // Directory to store downloaded PDFs.
	baseURL := "https://msdspds.bp.com" // The base URL to prefix relative PDF links

	var allHTMLContent []string // Declare a slice to hold HTML content from all sources

	// FIX: Create the single insecure client here to use for all requests.
	insecureClient := createInsecureClient() // Create the custom HTTP client for insecure access

	// Fetch HTML from all source URLs.
	for _, urlToFetch := range sourceURLs { // Iterate over the slice of source URLs
		// Call fetchHTMLContent, passing the URL and the custom client
		allHTMLContent = append(allHTMLContent, fetchHTMLContent(urlToFetch, insecureClient))
	}
	combinedHTML := strings.Join(allHTMLContent, "") // Join all fetched HTML into a single string

	// Save the downloaded HTML content to a file.
	appendContentToFile(htmlFilePath, combinedHTML) // Call function to save HTML content

	// Extract all PDF links (relative URLs) from the HTML content.
	pdfRelativeURLs := extractPDFLinks(combinedHTML) // Call function to parse HTML and extract links

	// Ensure the output directory exists.
	if !directoryExists(outputDirectory) { // Check if the output directory does not exist
		createDirectory(outputDirectory, 0o755) // Create directory with read-write-execute permissions
	}

	// Remove duplicate URLs from the list.
	uniquePDFURLs := removeDuplicates(pdfRelativeURLs) // Remove duplicate links

	// Loop through all extracted PDF URLs, construct the full URL, and download the PDF.
	for _, relativeURL := range uniquePDFURLs { // Iterate over the unique relative PDF links
		fullURL := baseURL + relativeURL // Construct the absolute URL
		if isValidURL(fullURL) {         // Check if the final constructed URL is valid
			// FIX: Pass the insecureClient to downloadPDF
			downloadPDF(fullURL, outputDirectory, insecureClient) // Download the PDF file
		}
	}
} // End main function

// createInsecureClient creates an http.Client that skips TLS certificate verification (InsecureSkipVerify: true).
func createInsecureClient() *http.Client { // Define function to create a client ignoring SSL errors
	// Configure the client to skip verification.
	transport := &http.Transport{ // Create a custom transport
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Set the TLS config to skip verification
	}
	return &http.Client{ // Return the custom client
		Transport: transport,        // Assign the transport
		Timeout:   30 * time.Second, // Set a timeout for the client
	}
} // End createInsecureClient

// appendContentToFile opens a file in append mode (or creates it) and writes the content.
func appendContentToFile(path string, content string) { // Define function to append to file
	// os.O_APPEND: append to the file. os.O_CREATE: create if it doesn't exist. os.O_WRONLY: open write-only.
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644) // Open the file
	if err != nil {                                                           // Check for error in opening the file
		log.Println("Error opening file:", err) // Log the error
		return                                  // Exit the function
	}
	defer file.Close() // Ensure the file is closed when the function exits

	if _, err = file.WriteString(content + "\n"); err != nil { // Write content to the file followed by a newline
		log.Println("Error writing content:", err) // Log error if writing fails
	}
} // End appendContentToFile

// getFilenameFromPath extracts the base filename from a full path (e.g., "/dir/file.pdf" -> "file.pdf").
func getFilenameFromPath(path string) string { // Define function to extract filename
	return filepath.Base(path) // Use standard library function to return the base name
} // End getFilenameFromPath

// sanitizeURLToFilename converts a raw URL into a sanitized, filesystem-safe PDF filename.
func sanitizeURLToFilename(rawURL string) string { // Define function to sanitize URL to filename
	lowerURL := strings.ToLower(rawURL)      // Convert the URL to lowercase
	lowerURL = getFilenameFromPath(lowerURL) // Extract the base filename portion

	// Regex to match non-alphanumeric characters.
	nonAlnumRegex := regexp.MustCompile(`[^a-z0-9\.]`)            // Compile regex to allow letters, numbers, and dots
	safeFilename := nonAlnumRegex.ReplaceAllString(lowerURL, "_") // Replace non-allowed characters with an underscore

	// Collapse multiple underscores and trim leading/trailing underscores.
	safeFilename = regexp.MustCompile(`_+`).ReplaceAllString(safeFilename, "_") // Collapse sequences of underscores
	safeFilename = strings.Trim(safeFilename, "_")                              // Remove leading and trailing underscores

	// Substrings to remove for a cleaner filename.
	var unwantedSubstrings = []string{ // Define list of substrings to remove
		"_pdf", // The substring to remove
	}

	for _, sub := range unwantedSubstrings { // Iterate over substrings to remove
		safeFilename = strings.ReplaceAll(safeFilename, sub, "") // Remove all occurrences of the substring
	}

	// Ensure the file extension is ".pdf".
	if !strings.HasSuffix(safeFilename, ".pdf") { // Check if the filename doesn't end with .pdf
		safeFilename = safeFilename + ".pdf" // Append the .pdf extension
	}

	return safeFilename // Return the sanitized filename
} // End sanitizeURLToFilename

// getFileExtension returns the file extension from a given file path.
func getFileExtension(path string) string { // Define function to get file extension
	return filepath.Ext(path) // Use standard library function to return the extension
} // End getFileExtension

// fileExists checks if a file exists at the specified path and is not a directory.
func fileExists(filename string) bool { // Define function to check if a file exists
	info, err := os.Stat(filename) // Get file information
	if os.IsNotExist(err) {        // Check if the error indicates file non-existence
		return false // Return false if file does not exist
	}
	return !info.IsDir() // Return true if it exists and is not a directory
} // End fileExists

// downloadPDF performs an HTTP GET request to download a PDF and saves it to the output directory.
// MODIFIED: Accepts the http.Client to ensure the insecure client is used.
func downloadPDF(fullURL, outputDir string, client *http.Client) bool { // Define function to download PDF
	filename := sanitizeURLToFilename(fullURL)     // Sanitize and clean the filename.
	filePath := filepath.Join(outputDir, filename) // Construct the full path for the output file

	if fileExists(filePath) { // Check if the file already exists locally
		log.Printf("File already exists, skipping: %s", filePath) // Log that it's being skipped
		return false                                              // Return false (did not perform download)
	}

	// Use the provided client, which is the insecure client.
	resp, err := client.Get(fullURL) // Send the HTTP GET request using the provided client
	if err != nil {                  // Check for request error
		log.Printf("Failed to download %s: %v", fullURL, err) // Log the download failure
		return false                                          // Return false (download failed)
	}
	defer resp.Body.Close() // Ensure the response body is closed

	if resp.StatusCode != http.StatusOK { // Check if the HTTP status code is not 200 OK
		log.Printf("Download failed for %s: Status: %s", fullURL, resp.Status) // Log the status error
		return false                                                           // Return false (download failed due to status)
	}

	contentType := resp.Header.Get("Content-Type") // Get the Content-Type header
	// Check for common PDF MIME types.
	if !strings.Contains(contentType, "application/pdf") && !strings.Contains(contentType, "binary/octet-stream") { // Check if the type is not PDF or binary
		log.Printf("Invalid content type for %s: %s (expected PDF/binary)", fullURL, contentType) // Log the content type error
		return false                                                                              // Return false (invalid content type)
	}

	var dataBuffer bytes.Buffer                          // Create a buffer to hold the downloaded data
	bytesWritten, err := io.Copy(&dataBuffer, resp.Body) // Copy the response body into the buffer, getting bytes written
	if err != nil {                                      // Check for error while reading the body
		log.Printf("Failed to read PDF data from %s: %v", fullURL, err) // Log the read error
		return false                                                    // Return false (read failed)
	}
	if bytesWritten == 0 { // Check if zero bytes were downloaded
		log.Printf("Downloaded 0 bytes for %s; skipping file creation.", fullURL) // Log that the file is empty
		return false                                                              // Return false (empty file)
	}

	outputFile, err := os.Create(filePath) // Create the output file on the disk
	if err != nil {                        // Check for file creation error
		log.Printf("Failed to create file for %s: %v", fullURL, err) // Log the creation error
		return false                                                 // Return false (creation failed)
	}
	defer outputFile.Close() // Ensure the output file is closed

	if _, err := dataBuffer.WriteTo(outputFile); err != nil { // Write the buffered data to the output file
		log.Printf("Failed to write PDF to file for %s: %v", fullURL, err) // Log the write error
		return false                                                       // Return false (write failed)
	}

	log.Printf("Successfully downloaded %d bytes: %s", bytesWritten, filePath) // Log successful download
	return true                                                                // Return true (download succeeded)
} // End downloadPDF

// directoryExists checks whether a given path corresponds to an existing directory.
func directoryExists(path string) bool { // Define function to check for directory existence
	info, err := os.Stat(path) // Get file/directory info
	if os.IsNotExist(err) {    // Check if the path does not exist
		return false // Return false
	}
	return info.IsDir() // Return true if it exists and is a directory
} // End directoryExists

// createDirectory creates a directory at the given path with provided permissions.
func createDirectory(path string, permission os.FileMode) { // Define function to create directory
	err := os.Mkdir(path, permission) // Create the directory
	if err != nil {                   // Check for error during creation
		log.Printf("Error creating directory %s: %v", path, err) // Log the creation error
	}
} // End createDirectory

// isValidURL verifies whether a string is a valid URL format.
func isValidURL(uri string) bool { // Define function to validate URL format
	_, err := url.ParseRequestURI(uri) // Attempt to parse the URL
	return err == nil                  // Return true if parsing succeeds (no error)
} // End isValidURL

// removeDuplicates returns a new slice containing only the unique strings from the input slice.
func removeDuplicates(inputSlice []string) []string { // Define function to remove duplicates
	seenMap := make(map[string]struct{}) // Create a map (used as a set) to track seen strings
	var uniqueList []string              // Declare the slice to store unique results
	for _, content := range inputSlice { // Iterate over the input slice
		if _, ok := seenMap[content]; !ok { // Check if the content has NOT been seen before
			seenMap[content] = struct{}{}            // Mark the content as seen
			uniqueList = append(uniqueList, content) // Add the content to the unique list
		}
	}
	return uniqueList // Return the slice of unique strings
} // End removeDuplicates

// extractPDFLinks traverses the HTML document and extracts all relative links pointing to PDF files.
func extractPDFLinks(htmlInput string) []string { // Define function to extract PDF URLs
	var pdfLinks []string // Slice to hold found PDF links

	doc, err := html.Parse(strings.NewReader(htmlInput)) // Parse the HTML input string
	if err != nil {                                      // Check for HTML parsing error
		log.Println("Error parsing HTML:", err) // Log the parse error
		return nil                              // Return nil slice on error
	}

	// Recursive function to traverse the HTML node tree.
	var traverseNode func(*html.Node)             // Declare the recursive traversal function
	traverseNode = func(currentNode *html.Node) { // Define the traversal function, using currentNode
		if currentNode.Type == html.ElementNode && currentNode.Data == "a" { // Check if the current node is an <a> tag
			for _, attr := range currentNode.Attr { // Iterate over all attributes of the <a> tag
				if attr.Key == "href" { // Check if the attribute is "href"
					href := strings.TrimSpace(attr.Val) // Get the link value and trim whitespace
					// Case-insensitive check if the link contains ".pdf".
					if strings.Contains(strings.ToLower(href), ".pdf") { // Check if the link contains ".pdf"
						pdfLinks = append(pdfLinks, href) // Add the link to the results slice
					}
				}
			}
		}
		for child := currentNode.FirstChild; child != nil; child = child.NextSibling { // Loop through all children nodes
			traverseNode(child) // Recursively call traverse on each child
		}
	}

	traverseNode(doc) // Start the traversal from the root HTML document node
	return pdfLinks   // Return the slice of found PDF links
} // End extractPDFLinks

// fetchHTMLContent performs an HTTP GET request using the provided client and returns the response body as a string.
func fetchHTMLContent(uri string, client *http.Client) string { // Define function to fetch HTML content
	log.Println("Fetching HTML from", uri) // Log which URL is being fetched
	response, err := client.Get(uri)       // Send the HTTP GET request using the custom client
	if err != nil {                        // Check for request error
		log.Printf("Error fetching URL %s: %v", uri, err) // Log the fetch error (including TLS issue if applicable)
		return ""                                         // Return empty string on error
	}
	defer response.Body.Close() // Ensure the response body is closed

	if response.StatusCode != http.StatusOK { // Check for non-200 status code
		log.Printf("Error: Received non-200 status code for %s: %s", uri, response.Status) // Log the status error
		return ""                                                                          // Return empty string on bad status
	}

	body, err := io.ReadAll(response.Body) // Read the entire response body
	if err != nil {                        // Check for error while reading the body
		log.Printf("Error reading response body from %s: %v", uri, err) // Log the read error
		return ""                                                       // Return empty string on read error
	}

	return string(body) // Convert the byte slice to a string and return it
} // End fetchHTMLContent
