import requests
import os
from dotenv import load_dotenv

def get_permanent_page_token():
    # Load environment variables
    load_dotenv()
    
    # Your credentials
    app_id = os.getenv('FACEBOOK_APP_ID')
    app_secret = os.getenv('FACEBOOK_APP_SECRET')
    page_id = os.getenv('FACEBOOK_PAGE_ID')
    short_lived_token = input("Enter your short-lived token from Graph API Explorer: ")
    
    # Step 1: Convert to Long-lived Token
    print("\nConverting to long-lived token...")
    url = "https://graph.facebook.com/v18.0/oauth/access_token"
    params = {
        'grant_type': 'fb_exchange_token',
        'client_id': app_id,
        'client_secret': app_secret,
        'fb_exchange_token': short_lived_token
    }
    
    response = requests.get(url, params=params)
    if response.status_code != 200:
        print("Error getting long-lived token:", response.text)
        return
        
    long_lived_token = response.json()['access_token']
    print("Successfully got long-lived token")
    
    # Step 2: Get Permanent Page Token
    print("\nGetting permanent page token...")
    url = f"https://graph.facebook.com/v18.0/{page_id}"
    params = {
        'fields': 'access_token',
        'access_token': long_lived_token
    }
    
    response = requests.get(url, params=params)
    if response.status_code != 200:
        print("Error getting page token:", response.text)
        return
        
    page_token = response.json()['access_token']
    print("\nSuccessfully got permanent page token!")
    
    # Step 3: Verify token
    print("\nVerifying token...")
    url = "https://graph.facebook.com/v18.0/debug_token"
    params = {
        'input_token': page_token,
        'access_token': f"{app_id}|{app_secret}"
    }
    
    response = requests.get(url, params=params)
    if response.status_code != 200:
        print("Error verifying token:", response.text)
        return
        
    token_info = response.json()['data']
    print("\nToken Information:")
    print(f"Type: {token_info.get('type', 'Unknown')}")
    print(f"App ID: {token_info.get('app_id', 'Unknown')}")
    print(f"Expires: {'Never' if token_info.get('expires_at', 0) == 0 else token_info['expires_at']}")
    
    # Return the token
    return page_token

if __name__ == "__main__":
    token = get_permanent_page_token()
    if token:
        print("\nYour permanent page token (save this securely):")
        print(token)
        
        # Optional: Save to .env file
        with open('.env', 'a') as f:
            f.write(f'\nFACEBOOK_PAGE_TOKEN={token}\n')
        print("\nToken has been appended to your .env file")