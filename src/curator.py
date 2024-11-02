import feedparser
from googletrans import Translator
from datetime import datetime
import json
import time
import random

class AIContentCurator:
    def __init__(self):
        self.translator = Translator()
        self.feeds = [
            'https://techcrunch.com/tag/artificial-intelligence/feed/',
            'https://venturebeat.com/category/ai/feed/',
        ]
        
        # Templates for post variety
        self.templates = [
            "🤖 {headline}\n\n💡 {key_point}\n\n👉 Lee más: {link}\n\n#IA #Innovación #Tecnología",
            
            "📱 Última hora en IA:\n\n{headline}\n\n🔑 Lo importante:\n{key_point}\n\n#InteligenciaArtificial #Tech",
            
            "🔮 El futuro es hoy:\n\n{headline}\n\n✨ {key_point}\n\n#TechNews #IA #Innovación",
            
            "💼 Para empresas:\n\n{headline}\n\n📌 {key_point}\n\n#NegociosDigitales #IA",
        ]
        
    def extract_key_point(self, summary):
        """Extract a single key point from article summary (1-2 sentences)"""
        sentences = summary.split('.')
        return '. '.join(sentences[:2]) + '.'
    
    def create_post(self, article):
        """Create a social media post from article"""
        try:
            # Translate title and extract key point
            headline = self.translator.translate(
                article.title, 
                src='en', 
                dest='es'
            ).text
            
            key_point = self.translator.translate(
                self.extract_key_point(article.summary), 
                src='en', 
                dest='es'
            ).text
            
            # Select random template
            template = random.choice(self.templates)
            
            # Create post
            post = template.format(
                headline=headline,
                key_point=key_point,
                link=article.link
            )
            
            return post
            
        except Exception as e:
            print(f"Error creating post: {str(e)}")
            return None
    
    def generate_posts(self, num_posts=5):
        """Generate multiple posts"""
        posts = []
        
        for feed_url in self.feeds:
            feed = feedparser.parse(feed_url)
            
            for entry in feed.entries[:num_posts]:
                post = self.create_post(entry)
                if post:
                    posts.append({
                        'post_content': post,
                        'original_link': entry.link,
                        'created_at': datetime.now().isoformat()
                    })
                time.sleep(1)  # Avoid hitting API limits
                
        return posts

# Example usage
if __name__ == "__main__":
    curator = AIContentCurator()
    posts = curator.generate_posts(num_posts=3)
    
    # Print example posts
    for i, post in enumerate(posts, 1):
        print(f"\nPost {i}:")
        print("-" * 50)
        print(post['post_content'])
        print("-" * 50)