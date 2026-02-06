#!/bin/bash

# Configuration
API_URL="http://localhost:8080/api/v1/projects"
# Note: This token is taken from your prompt. If it expires, update it here.
AUTH_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiNjk4NTQ5MjEyZGE0M2YyYTEyNGMxNzA3IiwiZXhwIjoxNzcwNDI5MDk4LCJpYXQiOjE3NzAzNDI2OTh9.MYCJElhswjJndDpUicmhvxFNB0ktvsKKt2B2S4H_LWk"

# Mock Data Arrays
BRANDS=("Dell" "HP" "Apple" "Razer" "MSI" "Lenovo" "Acer" "Sony" "Samsung" "Logitech")
DOMAINS=("dell.com" "hp.com" "apple.com" "razer.com" "msi.com" "lenovo.com" "acer.com" "sony.com" "samsung.com" "logitech.com")
HANDLES=("@dell" "@hp" "@apple" "@razer" "@msigaming" "@lenovo" "@acer" "@sony" "@samsung" "@logitech")
SCAN_fREQUENCY=5

echo "Starting creation of 10 mock projects..."
echo "----------------------------------------"

for i in {0..9}; do
  BRAND=${BRANDS[$i]}
  DOMAIN=${DOMAINS[$i]}
  HANDLE=${HANDLES[$i]}
  
  # Dynamic JSON Construction
  # We use a heredoc (EOF) variable to keep the JSON structure readable and safe
  JSON_DATA=$(cat <<EOF
{
  "name": "${BRAND} Global Sentiment",
  "description": "Tracking brand health and public perception for ${BRAND} devices",
  "brand_name": "${BRAND}",
  "primary_domain": "${DOMAIN}",
  "official_handles": [
    {
      "platform": "twitter",
      "handle": "${HANDLE}",
      "verified": true
    }
  ],
  "monitoring_config": {
    "keywords": [
      "${BRAND} laptop",
      "${BRAND} support",
      "${BRAND} performance"
    ],
    "hashtags": [
      "#${BRAND}",
      "#tech",
      "#gadgets"
    ],
    "scan_frequency": ${SCAN_fREQUENCY},
    "handles": [
      "${HANDLE}"
    ],
    "platforms": [
      "twitter",
      "reddit"
    ],
    "enable_typosquatting": true,
    "enable_impersonating": true,
    "typosquatting_scan_freq": 24
  },
  "alert_config": {
    "email_recipients": [
      "admin@${DOMAIN}",
      "monitoring@tech-agency.com"
    ],
    "viral_threshold": 100,
    "negative_spike_threshold": 10,
    "spike_window_minutes": 5,
    "slack_web_hook_url": ""
  }
}
EOF
)

  echo "Creating Project $((i+1))/10: ${BRAND}..."

  curl -s -o /dev/null -w "%{http_code}" -X POST "${API_URL}" \
    -H 'Accept: application/json, text/plain, */*' \
    -H 'Accept-Language: en-US,en;q=0.9' \
    -H "Authorization: Bearer ${AUTH_TOKEN}" \
    -H 'Connection: keep-alive' \
    -H 'Content-Type: application/json' \
    -H 'Origin: http://localhost:5173' \
    -H 'Referer: http://localhost:5173/' \
    -H 'Sec-Fetch-Dest: empty' \
    -H 'Sec-Fetch-Mode: cors' \
    -H 'Sec-Fetch-Site: same-site' \
    -H 'User-Agent: Mozilla/5.0 (Script/Bash)' \
    --data "${JSON_DATA}"

  # Add a newline for clean output
  echo " - Done"
  
  # Optional: slight pause to prevent overwhelming the server
  sleep 0.2
done

echo "----------------------------------------"
echo "All projects created successfully."