import requests
import os
from datetime import datetime
from dotenv import load_dotenv

def verify_token(token):
    """Verify token details and expiration"""
    load_dotenv()
    
    app_id = os.getenv('FACEBOOK_APP_ID')
    app_secret = os.getenv('FACEBOOK_APP_SECRET')
    
    # Debug token endpoint
    url = "https://graph.facebook.com/v18.0/debug_token"
    params = {
        'input_token': token,
        'access_token': f"{app_id}|{app_secret}"  # App access token
    }
    
    try:
        response = requests.get(url, params=params)
        data = response.json()['data']
        
        print("\nToken Information:")
        print("-" * 50)
        print(f"Type: {data.get('type', 'Unknown')}")
        print(f"App ID: {data.get('app_id', 'Unknown')}")
        
        # Check expiration
        expires_at = data.get('expires_at', 0)
        if expires_at == 0:
            print("Expiration: Never (Permanent Token) ✅")
        else:
            expiry_date = datetime.fromtimestamp(expires_at)
            print(f"Expiration: {expiry_date} ❌")
            print(f"Token will expire in {(expiry_date - datetime.now()).days} days")
        
        # Check if it's a page token
        print(f"Is Page Token: {'Yes ✅' if data.get('type') == 'PAGE' else 'No ❌'}")
        
        # Test actual permissions
        test_url = "https://graph.facebook.com/v18.0/me"
        test_response = requests.get(test_url, params={'access_token': token})
        if test_response.status_code == 200:
            print("Token Active: Yes ✅")
            print(f"Connected to Page: {test_response.json().get('name', 'Unknown')}")
        else:
            print("Token Active: No ❌")
        
        return data
        
    except Exception as e:
        print(f"Error verifying token: {str(e)}")
        return None

if __name__ == "__main__":
    token = os.getenv('FACEBOOK_PAGE_TOKEN')
    if not token:
        token = input("Enter the token to verify: ")
    
    result = verify_token(token)
    
    if result:
        print("\nSummary:")
        print("-" * 50)
        if result.get('type') == 'PAGE' and result.get('expires_at', 0) == 0:
            print("✅ This is a permanent page access token!")
        else:
            print("❌ This is NOT a permanent page access token!")