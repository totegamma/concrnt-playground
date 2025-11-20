import { useState } from 'react'
import './App.css'

interface Document {
    key: string;
    value: string

    reference?: string;

    signer: string;
    keyID?: string

    owner: string;

    type: string;
    schema: string;

    signedAt: Date;
}

function App() {
    const [key, setKey] = useState<string>("");
    const [draft, setDraft] = useState<string>("");
    const [username, setUsername] = useState<string>("user000");
    const [recordURI, setRecordURI] = useState<string>("");
    const [response, setResponse] = useState<string>("");

    const commitDocument = (doc: Document) => {
        const commit = {
            document: JSON.stringify(doc),
            signature: "signature_placeholder"
        }

        fetch('http://localhost:8000/commit', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(commit)
        })
    }

    return (
        <>
            <h3>key:</h3>
            <input
                type="text"
                value={key}
                onChange={(e) => setKey(e.target.value)}
            />
            <h3>value:</h3>
            <input
                type="text"
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
            />
            <h3>username:</h3>
            <input
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
            />
            <button
                onClick={() => {
                    commitDocument({
                        key: key,
                        value: draft,

                        signer: username,
                        owner: username,

                        type: "create",
                        schema: "https://example.com/schemas/message-v1.json",

                        signedAt: new Date(),
                    })
                }}
            >
                Commit Document
            </button>

            <hr />
            <h3>Record URI:</h3>
            <input
                type="text"
                value={recordURI}
                onChange={(e) => setRecordURI(e.target.value)}
            />
            <button
                onClick={() => {
                    fetch(`http://localhost:8000/resource/${recordURI}`)
                        .then(res => res.json())
                        .then(data => {
                            setResponse(JSON.stringify(data, null, 2))
                        })
                }}
            >
                Fetch Record
            </button>
            <pre>{response}</pre>
        </>
    )
}

export default App
