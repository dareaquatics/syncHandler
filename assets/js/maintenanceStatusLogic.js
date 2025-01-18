(function () {
    function checkMaintenanceStatus() {
        fetch('/maintenanceStatusToggle.json?nocache=' + new Date().getTime())
            .then(response => response.json())
            .then(data => {
                if (data.maintenanceMode) {
                    displayMaintenancePage();
                }
            })
            .catch(error => console.error('Error checking maintenance status:', error));
    }

    function displayMaintenancePage() {
        document.body.innerHTML = `
      <style>
        body {
            font-family: Arial, sans-serif;
            background-color: #f0f0f0;
            margin: 0;
            padding: 20px;
            color: #333;
        }
        .container {
            max-width: 800px;
            margin: 0 auto;
            background-color: white;
            padding: 20px;
            border-radius: 5px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        h1 {
            font-size: 24px;
            margin-top: 0;
        }
        .subtitle {
            font-size: 16px;
            color: #666;
            margin-bottom: 20px;
        }
        .browser-frame {
            border: 1px solid #ccc;
            border-radius: 5px;
            padding: 20px;
            margin-bottom: 20px;
        }
        .error-icon {
            width: 100px;
            height: 100px;
            background-color: #d9534f;
            border-radius: 50%;
            display: flex;
            justify-content: center;
            align-items: center;
            margin: 0 auto 20px;
        }
        .error-icon::before {
            content: "×";
            color: white;
            font-size: 80px;
            font-weight: bold;
        }
        .info-columns {
            display: flex;
            justify-content: space-between;
        }
        .info-column {
            flex-basis: 48%;
        }
        h2 {
            font-size: 18px;
        }
        p {
            font-size: 14px;
            line-height: 1.5;
        }
        .status-link {
            display: block;
            text-align: center;
            margin-top: 20px;
            font-size: 16px;
            color: #0056b3;
            text-decoration: none;
        }
        .status-link:hover {
            text-decoration: underline;
        }
        .footer {
            font-size: 12px;
            color: #888;
            margin-top: 20px;
            text-align: center;
        }
      </style>
      <div class="container">
        <h1>503 Service Unavailable</h1>
        <p class="subtitle">You are unable to access dareaquatics.com</p>
        <div class="browser-frame">
            <div class="error-icon"></div>
        </div>
        <div class="info-columns">
            <div class="info-column">
                <h2>What's happening?</h2>
                <p>We are currently performing scheduled maintenance on our website to improve your experience. This maintenance is necessary to ensure we can continue to provide you with the best possible service.</p>
            </div>
            <div class="info-column">
                <h2>When will the site be back?</h2>
                <p>We expect the maintenance to be completed soon. We appreciate your patience and understanding. Please check our status page for the most up-to-date information on our progress.</p>
            </div>
        </div>
        <a href="https://status.dareaquatics.com" class="status-link">Check our Status Page</a>
        <div class="footer">
            © 2024 DARE Aquatics • Maintenance ID: ${Date.now()}
        </div>
      </div>
    `;
    }

    checkMaintenanceStatus();
})();
