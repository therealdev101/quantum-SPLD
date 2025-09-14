const express = require("express")
const app = express()
const port = 3000

// Middleware
app.use(express.json())

// In-memory storage for enodes
let enodes = []

// Authentication token - Updated for private network
const ACCESS_TOKEN = 'private-network-1757346718388-suqw4gu5e'
console.log("New Access Token:", ACCESS_TOKEN)

// Middleware to check authorization
function authenticate(req, res, next) {
  const token = req.headers.authorization
  if (token !== ACCESS_TOKEN) {
    return res.status(401).json({ error: "Unauthorized" })
  }
  next()
}

// POST endpoint to add enode
app.post("/post-enode", authenticate, (req, res) => {
  const { enode } = req.body

  if (!enode) {
    return res.status(400).json({ error: "Enode is required" })
  }

  if (!enode.startsWith("enode://") && !enode.startsWith("enr:-")) {
    return res.status(400).json({
      error: "Invalid enode format. Must start with 'enode://' or 'enr:-'",
    })
  }

  // Add enode if not already present
  if (!enodes.includes(enode)) {
    enodes.push(enode)
    console.log(`Added enode: ${enode}`)
  }

  res.json({ success: true, message: "Enode added successfully" })
})

// GET endpoint to retrieve all enodes
app.get("/get-enode", (req, res) => {
  res.json(enodes)
})

// GET endpoint to retrieve all enodes without authentication
app.get("/get-enodes-public", (req, res) => {
  res.json(enodes)
})

// Health check endpoint
app.get("/health", (req, res) => {
  res.json({
    status: "healthy",
    enodeCount: enodes.length,
    timestamp: new Date().toISOString(),
  })
})

app.get("/", (req, res) => {
  res.send(`
    <!DOCTYPE html>
    <html>
    <head>
        <title>Enode Manager</title>
        <style>
            body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
            .container { background: #f5f5f5; padding: 20px; border-radius: 8px; margin: 10px 0; }
            input[type="text"] { width: 100%; padding: 10px; margin: 10px 0; border: 1px solid #ddd; border-radius: 4px; }
            button { background: #007cba; color: white; padding: 10px 20px; border: none; border-radius: 4px; margin: 5px; cursor: pointer; }
            button:hover { background: #005a87; }
            .result { margin: 20px 0; padding: 15px; border-radius: 4px; display: none; }
            .success { background: #d4edda; border: 1px solid #c3e6cb; color: #155724; }
            .error { background: #f8d7da; border: 1px solid #f5c6cb; color: #721c24; }
            pre { background: #f8f9fa; padding: 10px; border-radius: 4px; overflow-x: auto; }
        </style>
    </head>
    <body>
        <h1>Enode Manager</h1>
        
        <div class="container">
            <h3>Add New Enode</h3>
            <input type="text" id="enodeInput" placeholder="Enter enode string..." />
            <button onclick="postEnode()">Add Enode</button>
            <button onclick="clearInput()">Clear</button>
        </div>
        
        <div class="container">
            <h3>Actions</h3>
            <button onclick="getEnodes()">Get All Enodes</button>
            <button onclick="getEnodesPublic()">Get All Enodes Publicly</button>
            <button onclick="getHealth()">Health Check</button>
        </div>
        
        <div id="result" class="result"></div>

        <script>
            const ACCESS_TOKEN = "${ACCESS_TOKEN}";
            
            function showResult(message, isError = false) {
                const resultDiv = document.getElementById("result");
                resultDiv.style.display = "block";
                resultDiv.className = "result " + (isError ? "error" : "success");
                resultDiv.innerHTML = message;
            }
            
            async function postEnode() {
                const enode = document.getElementById("enodeInput").value.trim();
                if (!enode) {
                    showResult("<strong>Error:</strong> Please enter an enode", true);
                    return;
                }
                
                try {
                    const response = await fetch("/post-enode", {
                        method: "POST",
                        headers: { "Content-Type": "application/json", "Authorization": ACCESS_TOKEN },
                        body: JSON.stringify({ enode })
                    });
                    
                    const result = await response.json();
                    
                    if (response.ok) {
                        showResult("<strong>Success:</strong> " + result.message);
                        document.getElementById("enodeInput").value = "";
                    } else {
                        showResult("<strong>Error:</strong> " + result.error, true);
                    }
                } catch (error) {
                    showResult("<strong>Network Error:</strong> " + error.message, true);
                }
            }
            
            async function getEnodes() {
                try {
                    const response = await fetch("/get-enode");
                    const result = await response.json();
                    
                    if (response.ok) {
                        const count = Array.isArray(result) ? result.length : 0;
                        showResult("<strong>Current Enodes (" + count + "):</strong><pre>" + JSON.stringify(result, null, 2) + "</pre>");
                    } else {
                        showResult("<strong>Error:</strong> " + result.error, true);
                    }
                } catch (error) {
                    showResult("<strong>Network Error:</strong> " + error.message, true);
                }
            }
            
            async function getEnodesPublic() {
                try {
                    const response = await fetch("/get-enodes-public");
                    const result = await response.json();
                    
                    if (response.ok) {
                        const count = Array.isArray(result) ? result.length : 0;
                        showResult("<strong>Current Enodes Publicly (" + count + "):</strong><pre>" + JSON.stringify(result, null, 2) + "</pre>");
                    } else {
                        showResult("<strong>Error:</strong> " + result.error, true);
                    }
                } catch (error) {
                    showResult("<strong>Network Error:</strong> " + error.message, true);
                }
            }
            
            async function getHealth() {
                try {
                    const response = await fetch("/health");
                    const result = await response.json();
                    showResult("<strong>Health Status:</strong><pre>" + JSON.stringify(result, null, 2) + "</pre>");
                } catch (error) {
                    showResult("<strong>Network Error:</strong> " + error.message, true);
                }
            }
            
            function clearInput() {
                document.getElementById("enodeInput").value = "";
                showResult("<strong>Input cleared</strong>");
            }
        </script>
    </body>
    </html>
  `)
})

// Start server
app.listen(port, "0.0.0.0", () => {
  console.log(`Enode API server running on port ${port}`)
  console.log(`Health check: http://localhost:${port}/health`)
})

// Cleanup old enodes every 5 minutes
setInterval(() => {
  console.log(`Current enodes: ${enodes.length}`)
  // Keep only the last 10 enodes to prevent memory buildup
  if (enodes.length > 10) {
    enodes = enodes.slice(-10)
    console.log(`Cleaned up enodes, now have: ${enodes.length}`)
  }
}, 300000)
