import json
import feedparser
from googletrans import Translator
from datetime import datetime, timedelta
import random
from difflib import SequenceMatcher
import os
import re
from bs4 import BeautifulSoup
import html
import time
import argparse

class AIContentCurator:
    def __init__(self):
        self.translator = Translator()
        self.feeds = [
            'https://techcrunch.com/tag/artificial-intelligence/feed/',
            'https://venturebeat.com/category/ai/feed/',
            'https://www.artificialintelligence-news.com/feed/',
            'https://www.zdnet.com/topic/artificial-intelligence/rss.xml',
            'https://www.theverge.com/ai-artificial-intelligence/rss/index.xml',
            'https://www.marktechpost.com/feed/',
            'https://www.unite.ai/feed/',
            'https://www.xataka.com/tag/inteligencia-artificial/feed',
            'https://wwwhatsnew.com/category/inteligencia-artificial/feed/',
            'https://hipertextual.com/tag/inteligencia-artificial/feed'
        ]
        
        # Load post history from local file
        self.history_file = 'post_history.json'
        self.load_post_history()

    def load_post_history(self):
        """Load post history from local file"""
        try:
            if os.path.exists(self.history_file):
                with open(self.history_file, 'r', encoding='utf-8') as f:
                    self.post_history = json.load(f)
            else:
                self.post_history = []
        except Exception as e:
            print(f"Error loading post history: {str(e)}")
            self.post_history = []

    def save_post_history(self):
        """Save post history to local file"""
        try:
            seven_days_ago = datetime.now() - timedelta(days=7)
            self.post_history = [
                post for post in self.post_history 
                if datetime.fromisoformat(post['created_at']) > seven_days_ago
            ]
            
            with open(self.history_file, 'w', encoding='utf-8') as f:
                json.dump(self.post_history, f, ensure_ascii=False, indent=2)
        except Exception as e:
            print(f"Error saving post history: {str(e)}")

    def is_duplicate(self, title, threshold=0.85):
        """Check if article is too similar to previous posts"""
        for post in self.post_history:
            if 'original_title' in post:
                similarity = SequenceMatcher(None, title.lower(), post['original_title'].lower()).ratio()
                if similarity > threshold:
                    return True
        return False

    def clean_text(self, text):
            """Clean text from HTML and other unwanted content"""
            # Convert HTML entities
            text = html.unescape(text)
            
            # Remove HTML tags
            soup = BeautifulSoup(text, 'html.parser')
            text = soup.get_text()
            
            # Remove URLs
            text = re.sub(r'http[s]?://\S+', '', text)
            
            # Remove image credits and captions
            text = re.sub(r'(?i)imagen:.*?(?=\n|$)', '', text)  # Remove "Imagen: ..."
            text = re.sub(r'(?i)image:.*?(?=\n|$)', '', text)   # Remove "Image: ..."
            text = re.sub(r'(?i)\|.*?imagen.*?(?=\n|$)', '', text)  # Remove "| Imagen ..."
            text = re.sub(r'\|[^|]*?\|', '', text)  # Remove anything between pipes
            
            # Remove multiple newlines and spaces
            text = re.sub(r'\n+', '\n', text)
            text = re.sub(r' +', ' ', text)
            
            # Remove lines that are just credits or metadata
            lines = [line.strip() for line in text.split('\n')]
            lines = [line for line in lines if line and not any(x in line.lower() for x in ['imagen:', 'image:', 'photo:', 'foto:', 'crédito:', 'credit:'])]
            
            return ' '.join(lines).strip()

    def extract_key_point(self, summary):
        """Extract key point from summary"""
        # Clean the summary first
        summary = self.clean_text(summary)
        
        # Split into sentences and get first meaningful ones
        sentences = [s.strip() for s in summary.split('.') if s.strip()]
        key_sentences = [s for s in sentences[:2] if len(s.split()) > 3]
        
        if not key_sentences:
            return summary[:100] + "..."
            
        return '. '.join(key_sentences) + '.'

    def translate_with_retry(self, text, src='en', dest='es', max_retries=3):
            """Attempt translation with retries and proper error handling"""
            if not text:
                print("Empty text provided for translation")
                return None
                
            print(f"Attempting to translate: {text[:100]}...")  # Print first 100 chars
            
            for attempt in range(max_retries):
                try:
                    print(f"Translation attempt {attempt + 1}/{max_retries}")
                    translation = self.translator.translate(text, src=src, dest=dest)
                    
                    if translation and hasattr(translation, 'text') and translation.text:
                        print(f"Translation successful: {translation.text[:100]}...")
                        return translation.text
                        
                    print(f"Translation attempt {attempt + 1} failed: No valid translation returned")
                    
                except Exception as e:
                    print(f"Translation attempt {attempt + 1} failed with error: {str(e)}")
                    # Recreate translator object on error
                    self.translator = Translator()
                
                if attempt < max_retries - 1:
                    print("Waiting before retry...")
                    time.sleep(2)
            
            return None

    def create_post(self, entry):
        """Create a social media post"""
        try:
            if self.is_duplicate(entry.title):
                print("Duplicate post found, skipping...")
                return None
                
            # Simple language detection
            is_spanish = any(word in entry.title.lower() for word in ['la', 'el', 'los', 'las', 'en', 'con', 'para'])
            
            # Clean texts first
            clean_title = self.clean_text(entry.title)
            clean_summary = self.extract_key_point(entry.summary)

            print(f"\nProcessing article: {clean_title[:100]}...")
            print(f"Language detected: {'Spanish' if is_spanish else 'English'}")

            if is_spanish:
                headline = clean_title
                key_point = clean_summary
                print("Spanish content, no translation needed")
            else:
                print("\nTranslating title...")
                headline = self.translate_with_retry(clean_title)
                if not headline:
                    print("Title translation failed, skipping post")
                    return None

                print("\nTranslating summary...")
                key_point = self.translate_with_retry(clean_summary)
                if not key_point:
                    print("Summary translation failed, skipping post")
                    return None

            # Create simple post format
            post = f"{headline}\n\n{key_point}\n\nMás información: {entry.link}\n\n#IA #Tech #Innovación"
            print("\nPost created successfully!")
            
            return {
                'post_content': post,
                'original_link': entry.link,
                'original_title': entry.title,
                'created_at': datetime.now().isoformat(),
                'source': entry.get('feed', {}).get('title', 'Unknown Source')
            }
            
        except Exception as e:
            print(f"Error creating post: {str(e)}")
            import traceback
            print(traceback.format_exc())
            return None

    def generate_posts(self, num_posts=2):
        """Generate multiple posts"""
        posts = []
        all_entries = []
        
        print("Fetching feeds...")
        for feed_url in self.feeds:
            try:
                feed = feedparser.parse(feed_url)
                all_entries.extend(feed.entries)
                print(f"✓ {feed_url}")
            except Exception as e:
                print(f"✗ Error parsing {feed_url}: {str(e)}")
                continue
        
        print(f"\nFound {len(all_entries)} total entries")
        
        # Sort by date
        all_entries.sort(
            key=lambda x: getattr(x, 'published_parsed', datetime.now().timetuple()),
            reverse=True
        )
        
        # Generate posts
        entries_tried = 0
        for entry in all_entries:
            if len(posts) >= num_posts:
                break
            
            if entries_tried >= 10:  # Limit how many entries we try
                print("Tried too many entries without success, stopping")
                break
                
            entries_tried += 1
            print(f"\nProcessing entry {entries_tried}...")
            
            post = self.create_post(entry)
            if post:
                posts.append(post)
                self.post_history.append(post)
        
        # Save updated history
        self.save_post_history()
        
        return posts

# Add this at the bottom of the file, replace the current main
if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='AI Content Curator')
    parser.add_argument('--ignore-history', action='store_true', 
                      help='Ignore post history (for testing)')
    parser.add_argument('--clear-history', action='store_true',
                      help='Clear post history before running')
    args = parser.parse_args()

    curator = AIContentCurator()
    
    if args.clear_history:
        print("Clearing post history...")
        curator.post_history = []
        curator.save_post_history()
    
    # Modify the is_duplicate method to respect the ignore-history flag
    if args.ignore_history:
        print("Running with history check disabled...")
        curator.is_duplicate = lambda x, y=0: False
    
    posts = curator.generate_posts(num_posts=2)
    
    print(f"\nGenerated {len(posts)} posts:")
    for i, post in enumerate(posts, 1):
        print(f"\nPost {i}:")
        print("-" * 50)
        print(post['post_content'])
        print("-" * 50)