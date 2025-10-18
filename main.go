package main // Defines the main application package

import ( // Start import block
	"bytes"         // Imports the bytes package for buffer manipulation
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
	outputDir := "PDFs"                 // Directory to store downloaded PDFs.
	baseURL := "https://msdspds.bp.com" // The base URL to prefix relative PDF links

	var allHTMLContent []string // Declare a slice to hold HTML content from all sources

	// Fetch HTML from all source URLs.
	for _, u := range sourceURLs { // Iterate over the slice of source URLs
		allHTMLContent = append(allHTMLContent, fetchHTMLContent(u)) // Fetch content and append to slice
	}
	combinedHTML := strings.Join(allHTMLContent, "") // Join all fetched HTML into a single string

	// Save the downloaded HTML content to a file.
	appendAndWriteToFile(htmlFilePath, combinedHTML) // Call function to save HTML content

	// Extract all PDF links (relative URLs) from the HTML content.
	pdfRelativeURLs := extractPDFUrls(combinedHTML) // Call function to parse HTML and extract links

	// Ensure the output directory exists.
	if !directoryExists(outputDir) { // Check if the output directory does not exist
		createDirectory(outputDir, 0o755) // Create directory with read-write-execute permissions
	}

	// Remove duplicate URLs from the list.
	uniquePDFURLs := removeDuplicatesFromSlice(pdfRelativeURLs) // Remove duplicate links

	// Loop through all extracted PDF URLs, construct the full URL, and download the PDF.
	for _, relativeURL := range uniquePDFURLs { // Iterate over the unique relative PDF links
		fullURL := baseURL + relativeURL // Construct the absolute URL
		if isValidURL(fullURL) {         // Check if the final constructed URL is valid
			downloadPDF(fullURL, outputDir) // Download the PDF file
		}
	}
} // End main function

// appendAndWriteToFile opens a file in append mode (or creates it) and writes the content.
func appendAndWriteToFile(path string, content string) { // Define function to append to file
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
} // End appendAndWriteToFile

// getFilename extracts the base filename from a full path (e.g., "/dir/file.pdf" -> "file.pdf").
func getFilename(path string) string { // Define function to extract filename
	return filepath.Base(path) // Use standard library function to return the base name
} // End getFilename

// cleanURLToFilename converts a raw URL into a sanitized, filesystem-safe PDF filename.
func cleanURLToFilename(rawURL string) string { // Define function to sanitize URL to filename
	lower := strings.ToLower(rawURL) // Convert the URL to lowercase
	lower = getFilename(lower)       // Extract the base filename portion

	// Regex to match non-alphanumeric characters.
	reNonAlnum := regexp.MustCompile(`[^a-z0-9\.]`) // Compile regex to allow letters, numbers, and dots
	safe := reNonAlnum.ReplaceAllString(lower, "_") // Replace non-allowed characters with an underscore

	// Collapse multiple underscores and trim leading/trailing underscores.
	safe = regexp.MustCompile(`_+`).ReplaceAllString(safe, "_") // Collapse sequences of underscores
	safe = strings.Trim(safe, "_")                              // Remove leading and trailing underscores

	// Substrings to remove for a cleaner filename.
	var substringsToRemove = []string{ // Define list of substrings to remove
		"_pdf", // The substring to remove
	}

	for _, sub := range substringsToRemove { // Iterate over substrings to remove
		safe = strings.ReplaceAll(safe, sub, "") // Remove all occurrences of the substring
	}

	// Ensure the file extension is ".pdf".
	if !strings.HasSuffix(safe, ".pdf") { // Check if the filename doesn't end with .pdf
		safe = safe + ".pdf" // Append the .pdf extension
	}

	return safe // Return the sanitized filename
} // End cleanURLToFilename

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
func downloadPDF(fullURL, outputDir string) bool { // Define function to download PDF
	filename := cleanURLToFilename(fullURL)        // Sanitize and clean the filename.
	filePath := filepath.Join(outputDir, filename) // Construct the full path for the output file

	if fileExists(filePath) { // Check if the file already exists locally
		log.Printf("File already exists, skipping: %s", filePath) // Log that it's being skipped
		return false                                              // Return false (did not perform download)
	}

	client := &http.Client{Timeout: 30 * time.Second} // Create an HTTP client with a timeout.

	resp, err := client.Get(fullURL) // Send the HTTP GET request
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

	var buf bytes.Buffer                     // Create a buffer to hold the downloaded data
	written, err := io.Copy(&buf, resp.Body) // Copy the response body into the buffer, getting bytes written
	if err != nil {                          // Check for error while reading the body
		log.Printf("Failed to read PDF data from %s: %v", fullURL, err) // Log the read error
		return false                                                    // Return false (read failed)
	}
	if written == 0 { // Check if zero bytes were downloaded
		log.Printf("Downloaded 0 bytes for %s; skipping file creation.", fullURL) // Log that the file is empty
		return false                                                              // Return false (empty file)
	}

	out, err := os.Create(filePath) // Create the output file on the disk
	if err != nil {                 // Check for file creation error
		log.Printf("Failed to create file for %s: %v", fullURL, err) // Log the creation error
		return false                                                 // Return false (creation failed)
	}
	defer out.Close() // Ensure the output file is closed

	if _, err := buf.WriteTo(out); err != nil { // Write the buffered data to the output file
		log.Printf("Failed to write PDF to file for %s: %v", fullURL, err) // Log the write error
		return false                                                       // Return false (write failed)
	}

	log.Printf("Successfully downloaded %d bytes: %s", written, filePath) // Log successful download
	return true                                                           // Return true (download succeeded)
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

// removeDuplicatesFromSlice returns a new slice containing only the unique strings from the input slice.
func removeDuplicatesFromSlice(slice []string) []string { // Define function to remove duplicates
	seen := make(map[string]struct{}) // Create a map (used as a set) to track seen strings
	var uniqueList []string           // Declare the slice to store unique results
	for _, content := range slice {   // Iterate over the input slice
		if _, ok := seen[content]; !ok { // Check if the content has NOT been seen before
			seen[content] = struct{}{}               // Mark the content as seen
			uniqueList = append(uniqueList, content) // Add the content to the unique list
		}
	}
	return uniqueList // Return the slice of unique strings
} // End removeDuplicatesFromSlice

// extractPDFUrls traverses the HTML document and extracts all relative links pointing to PDF files.
func extractPDFUrls(htmlInput string) []string { // Define function to extract PDF URLs
	var pdfLinks []string // Slice to hold found PDF links

	doc, err := html.Parse(strings.NewReader(htmlInput)) // Parse the HTML input string
	if err != nil {                                      // Check for HTML parsing error
		log.Println("Error parsing HTML:", err) // Log the parse error
		return nil                              // Return nil slice on error
	}

	// Recursive function to traverse the HTML node tree.
	var traverse func(*html.Node)   // Declare the recursive traversal function
	traverse = func(n *html.Node) { // Define the traversal function
		if n.Type == html.ElementNode && n.Data == "a" { // Check if the current node is an <a> tag
			for _, attr := range n.Attr { // Iterate over all attributes of the <a> tag
				if attr.Key == "href" { // Check if the attribute is "href"
					href := strings.TrimSpace(attr.Val) // Get the link value and trim whitespace
					// Case-insensitive check if the link ends with or contains ".pdf".
					if strings.Contains(strings.ToLower(href), ".pdf") { // Check if the link contains ".pdf"
						pdfLinks = append(pdfLinks, href) // Add the link to the results slice
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling { // Loop through all children nodes
			traverse(c) // Recursively call traverse on each child
		}
	}

	traverse(doc)   // Start the traversal from the root HTML document node
	return pdfLinks // Return the slice of found PDF links
} // End extractPDFUrls

// fetchHTMLContent performs an HTTP GET request and returns the response body as a string.
func fetchHTMLContent(uri string) string { // Define function to fetch HTML content
	log.Println("Fetching HTML from", uri) // Log which URL is being fetched
	response, err := http.Get(uri)         // Send the HTTP GET request
	if err != nil {                        // Check for request error
		log.Printf("Error fetching URL %s: %v", uri, err) // Log the fetch error
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
