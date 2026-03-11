This tool serves a webapp that allows a user to interact with an LLM to explain something.
The tool generates, stores and serves HTML pages containing explanations about topics the user is interested in.
The HTML files are structured into sections.
Javascript loaded onto each explanation adds functionality that allows the user to ask for further explanation
about the section.
The response from the AI backend can then provide an updated section that should be inserted into the HTML document
as a new version.
Users should have the ability to browser previous versions of the sections and there should be an easy way to reset 
the doc to be viewing the most recent of each section.

## Technologies

- The backend is written in Go
    - It uses Cobra and Viper for managing its startup command line and configuration.
    - Viper should be configured to fall back to the environment.
    - The server should only listen on localhost. It's assumed this application is running locally and should only listen for local connections.
    - The backend uses a sqlite database stored at ~/.aiexplains/database.sqlite to store threads and state.
    - Generated HTML files are stored in ~/.aiexplains/explanations/
    
- The frontend uses React and is served by the Go backend server. 
    - There is no additional express or other nodejs server for the frontend.

- LLMs
    - The backend should interact with Claude via the official Go SDK.

- The service is primarily designed to run locally, exec'ing the Claude CLI to leverage your already authenticated account.

## Code organization

- The go backend should be put in the ./backend directory
- The react frontend should be put in the ./frontend directory

## UX

- The home page is a time ordered index of explanations. Ordered by update time, not created time.
- Clicking one of the explanations loads the page.
- The page should be divided into sections using a "section" class on divs to identify each section.
- There should be UI to enable the user to ask additional questions about the section.
- The backend should then continue the session for the specific explanation, passing the users additional request with the existing section content.
- Additionally the backend will add to the prompt to request a response in HTML that can be inserted to replace the section in the doc.