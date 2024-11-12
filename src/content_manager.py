import random
from datetime import datetime
import logging
from typing import Dict, Optional
from .templates import image_content_templates

class ContentManager:
    def __init__(self):
        """Initialize ContentManager with templates and setup logging"""
        self.templates = image_content_templates
        self.setup_logging()
        
    def setup_logging(self):
        """Setup logging configuration"""
        logging.basicConfig(
            level=logging.INFO,
            format='%(asctime)s - %(levelname)s - %(message)s'
        )
        self.logger = logging.getLogger(__name__)

    def get_random_template(self, category: str) -> Dict:
        """Get random template from specified category"""
        try:
            if category not in self.templates:
                raise ValueError(f"Category {category} not found")
            return self.templates[category]
        except Exception as e:
            self.logger.error(f"Error getting template for category {category}: {str(e)}")
            raise

    def format_content(self, templates: Dict) -> Dict:
        """Format content using templates"""
        try:
            # Select random elements from each list
            title = random.choice(templates['titles'])
            content_template = random.choice(templates['content_templates'])
            
            # Fill in the template with random choices from other lists
            content_vars = {}
            for key in templates.keys():
                if key not in ['titles', 'content_templates'] and isinstance(templates[key], list):
                    content_vars[key.rstrip('s')] = random.choice(templates[key])
            
            content = content_template.format(**content_vars)
            
            return {
                'title': title,
                'content': content
            }
        except Exception as e:
            self.logger.error(f"Error formatting content: {str(e)}")
            raise

    def generate_image_post(self, category: Optional[str] = None) -> Dict:
        """
        Generate content for image post
        Args:
            category (str, optional): Specific category to use. If None, random category is chosen.
        Returns:
            Dict: Generated post content
        """
        try:
            # Select category
            if category is None:
                category = random.choice(list(self.templates.keys()))
            
            self.logger.info(f"Generating post for category: {category}")
            
            # Get templates for category
            templates = self.get_random_template(category)
            
            # Format content
            formatted_content = self.format_content(templates)
            
            # Create final post
            post = f"{formatted_content['title']}\n\n{formatted_content['content']}\n\n#IA #Innovación #NeuroCrow #TechMX"
            
            result = {
                'type': 'image',
                'content': post,
                'created_at': datetime.now().isoformat(),
                'category': category,
                'metadata': {
                    'template_used': category,
                    'generation_time': datetime.now().isoformat()
                }
            }
            
            self.logger.info("Post generated successfully")
            return result
            
        except Exception as e:
            self.logger.error(f"Error generating image post: {str(e)}")
            raise

    def get_available_categories(self) -> list:
        """Get list of available template categories"""
        return list(self.templates.keys())

    def validate_category(self, category: str) -> bool:
        """Validate if category exists"""
        return category in self.templates

    def get_category_stats(self, category: str) -> Dict:
        """
        Get statistics about a category's templates
        Returns count of variations available
        """
        if not self.validate_category(category):
            raise ValueError(f"Category {category} not found")
            
        templates = self.templates[category]
        
        return {
            'category': category,
            'title_variations': len(templates['titles']),
            'content_template_variations': len(templates['content_templates']),
            'variable_counts': {
                key: len(value) for key, value in templates.items()
                if isinstance(value, list) and key not in ['titles', 'content_templates']
            }
        }

    def generate_multiple_posts(self, count: int, unique_categories: bool = True) -> list:
        """
        Generate multiple posts
        Args:
            count (int): Number of posts to generate
            unique_categories (bool): Whether to use different categories for each post
        Returns:
            list: List of generated posts
        """
        posts = []
        used_categories = set()
        
        for _ in range(count):
            available_categories = [cat for cat in self.get_available_categories() 
                                 if not unique_categories or cat not in used_categories]
            
            if not available_categories:
                self.logger.warning("No more unique categories available")
                break
                
            category = random.choice(available_categories)
            post = self.generate_image_post(category)
            posts.append(post)
            used_categories.add(category)
            
        return posts

    def preview_template(self, category: str) -> Dict:
        """
        Preview a template's possible variations without generating a full post
        Useful for testing and verification
        """
        if not self.validate_category(category):
            raise ValueError(f"Category {category} not found")
            
        templates = self.templates[category]
        
        return {
            'category': category,
            'sample_title': random.choice(templates['titles']),
            'sample_content_template': random.choice(templates['content_templates']),
            'available_variables': {
                key: value for key, value in templates.items()
                if isinstance(value, list) and key not in ['titles', 'content_templates']
            }
        }

    def get_template_variables(self, category: str) -> Dict:
        """Get all available variables for a template category"""
        if not self.validate_category(category):
            raise ValueError(f"Category {category} not found")
            
        return self.templates[category]

    def get_post_preview(self, category: str = None) -> str:
        """Generate a post preview without metadata"""
        post = self.generate_image_post(category)
        return post['content']

if __name__ == "__main__":
    # Example usage and testing
    manager = ContentManager()
    
    # Generate a random post
    post = manager.generate_image_post()
    print("\nRandom Post:")
    print("-" * 50)
    print(post['content'])
    
    # Show available categories
    print("\nAvailable Categories:")
    print("-" * 50)
    for category in manager.get_available_categories():
        stats = manager.get_category_stats(category)
        print(f"{category}: {stats['title_variations']} titles, "
              f"{stats['content_template_variations']} templates")