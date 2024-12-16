from content_manager import ContentManager

def test_content_generation():
    manager = ContentManager()
    
    print("\nTesting different categories:")
    print("=" * 50)
    
    # Test each category
    for category in manager.get_available_categories():
        print(f"\nTesting category: {category}")
        print("-" * 50)
        post = manager.generate_image_post(category)
        print(post['content'])
        
    print("\nTesting random generation:")
    print("=" * 50)
    
    # Test random generation
    for i in range(3):
        print(f"\nRandom Post {i+1}:")
        print("-" * 50)
        post = manager.generate_image_post()
        print(post['content'])
        print(f"Category used: {post['category']}")

if __name__ == "__main__":
    test_content_generation()