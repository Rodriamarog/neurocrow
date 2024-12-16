import requests
import os
from dotenv import load_dotenv

def test_facebook_post():
    """Test posting to Facebook page"""
    load_dotenv()
    
    # Get credentials from .env
    access_token = os.getenv('FACEBOOK_PAGE_TOKEN')
    page_id = os.getenv('FACEBOOK_PAGE_ID')
    
    if not access_token or not page_id:
        print("Missing credentials in .env file!")
        return
        
    # Test message
    message = "🤖 ¡Prueba de publicación automática!\n\n" \
              "Este es un post de prueba desde la API de NeuroCrow.\n\n" \
              "#Test #IA #Automatización"
    
    # API endpoint
    url = f"https://graph.facebook.com/v18.0/{page_id}/feed"
    
    # Post data
    data = {
        'message': message,
        'access_token': access_token
    }
    
    try:
        # Make the post
        print("Attempting to post...")
        response = requests.post(url, data=data)
        
        # Check response
        if response.status_code == 200:
            post_id = response.json().get('id')
            print("\n✅ Post successful!")
            print(f"Post ID: {post_id}")
            print(f"You can view it at: https://facebook.com/{post_id}")
        else:
            print("\n❌ Post failed!")
            print("Error:", response.text)
            
    except Exception as e:
        print(f"Error making post: {str(e)}")

def test_image_post():
    """Test posting an image with text to Facebook page"""
    load_dotenv()
    
    access_token = os.getenv('FACEBOOK_PAGE_TOKEN')
    page_id = os.getenv('FACEBOOK_PAGE_ID')
    
    if not access_token or not page_id:
        print("Missing credentials in .env file!")
        return
    
    # Test message
    message = "🤖 ¡Prueba de publicación con imagen!\n\n" \
              "Este es un post de prueba con imagen desde la API de NeuroCrow.\n\n" \
              "#Test #IA #Automatización"
    
    # URL of a test image (replace with your image URL)
    image_url = "https://static.wikia.nocookie.net/airtv/images/9/9e/Sora_Anime.gif/revision/latest?cb=20121229045520"
    
    # API endpoint for photos
    url = f"https://graph.facebook.com/v18.0/{page_id}/photos"
    
    data = {
        'message': message,
        'access_token': access_token,
        'url': image_url  # Can also use local file with 'source' parameter
    }
    
    try:
        print("Attempting to post image...")
        response = requests.post(url, data=data)
        
        if response.status_code == 200:
            post_id = response.json().get('id')
            print("\n✅ Image post successful!")
            print(f"Post ID: {post_id}")
            print(f"You can view it at: https://facebook.com/{post_id}")
        else:
            print("\n❌ Image post failed!")
            print("Error:", response.text)
            
    except Exception as e:
        print(f"Error making image post: {str(e)}")

if __name__ == "__main__":
    #print("Testing text-only post...")
    #test_facebook_post()
    
    # Uncomment to test image post
    print("\nTesting image post...")
    test_image_post()