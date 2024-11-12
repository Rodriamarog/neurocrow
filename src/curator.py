import json
import feedparser
from dotenv import load_dotenv
import deepl
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
from langdetect import detect, LangDetectException

class AIContentCurator:
    def __init__(self):
        # Load environment variables
        load_dotenv()
        
        # Initialize DeepL translator
        auth_key = os.getenv('DEEPL_API_KEY')
        if not auth_key:
            raise ValueError("DEEPL_API_KEY not found in environment variables")
            
        self.translator = deepl.Translator(auth_key)

        # List of feeds
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
        
        # Initialize promotional content filters
        self.initialize_content_filters()
        
        # Load post history from local file
        self.history_file = 'post_history.json'
        self.load_post_history()

    def initialize_content_filters(self):
        """Initialize filters for promotional content"""
        # Strong indicators of promotional content
        self.promotional_patterns = [
            r'black friday',
            r'cyber monday',
            r'(?:best|top)\s+\d+',  # "best 10", "top 5", etc.
            r'(?:sale|deals?)(?:\s|$)',
            r'review(?:ing)?(?:\s|$)',
            r'buying guide',
            r'shop now',
            r'limited time',
            r'discount',
            r'offer(?:s)?(?:\s|$)',
            r'price(?:s)?(?:\s|$)',
            r'\$\d+',
            r'(?:save|saving)\s+\d+%',
            r'promo(?:tion)?(?:s)?(?:\s|$)',
            r'coupon(?:s)?(?:\s|$)',
            # Spanish equivalents
            r'oferta(?:s)?(?:\s|$)',
            r'descuento(?:s)?(?:\s|$)',
            r'promoción(?:es)?(?:\s|$)',
            r'mejor(?:es)?\s+\d+',
        ]
        
        # Context words that suggest promotional content when combined
        self.context_keywords = {
            'primary': [
                'buy', 'purchase', 'deal', 'save', 'offer', 'price',
                'comprar', 'precio', 'oferta', 'descuento'
            ],
            'secondary': [
                'now', 'today', 'limited', 'exclusive', 'special',
                'ahora', 'hoy', 'limitado', 'especial'
            ]
        }

    def is_promotional_content(self, title, summary):
        """
        Check if content is promotional using a moderate filter
        Returns: (bool, str) - (is_promotional, reason)
        """
        text = f"{title} {summary}".lower()
        
        # Check for strong promotional patterns
        for pattern in self.promotional_patterns:
            if re.search(pattern, text, re.IGNORECASE):
                return True, f"Matched promotional pattern: {pattern}"
        
        # Check for primary keywords
        primary_matches = sum(1 for word in self.context_keywords['primary'] if word in text)
        secondary_matches = sum(1 for word in self.context_keywords['secondary'] if word in text)
        
        # If we find multiple primary keywords or a combination with secondary keywords
        if primary_matches >= 2 or (primary_matches >= 1 and secondary_matches >= 2):
            return True, "Multiple promotional keywords detected"
        
        return False, ""

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
            # Keep only last 24 hours
            one_day_ago = datetime.now() - timedelta(days=1)
            self.post_history = [
                post for post in self.post_history 
                if datetime.fromisoformat(post['created_at']) > one_day_ago
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
            
            # Remove image descriptions and metadata patterns
            patterns_to_remove = [
                r'(?i)imagen:.*?(?=\n|$)',              # Spanish image captions
                r'(?i)image:.*?(?=\n|$)',               # English image captions
                r'(?i)\|.*?\|',                         # Content between pipes
                r'(?i)photo:.*?(?=\n|$)',              # Photo credits
                r'(?i)foto:.*?(?=\n|$)',               # Spanish photo credits
                r'(?i)crédito:.*?(?=\n|$)',            # Spanish credits
                r'(?i)credit:.*?(?=\n|$)',             # English credits
                r'(?i)source:.*?(?=\n|$)',             # Source attributions
                r'(?i)fuente:.*?(?=\n|$)',             # Spanish source attributions
                r'(?i)picture:.*?(?=\n|$)',            # Picture descriptions
                r'(?i)\[.*?\]',                        # Content in square brackets
                r'(?i)\(.*?\)',                        # Content in parentheses
                r'(?i)website\.',                      # Website references
                r'(?i)sitio web',                      # Spanish website references
                r'\|.*$',                              # Everything after a pipe
                r'Image:.*$',                          # Image descriptions
                r'^\s*\w+\'s\s+\w+\s+(?:website|site).*$',  # Website attributions
                r'^\s*\|.*$',                          # Lines starting with pipe
            ]
            
            # Apply all cleaning patterns
            for pattern in patterns_to_remove:
                text = re.sub(pattern, '', text)
            
            # Remove multiple newlines and spaces
            text = re.sub(r'\n+', ' ', text)
            text = re.sub(r'\s+', ' ', text)
            
            # Split into lines and filter out metadata-like lines
            lines = [line.strip() for line in text.split('.') if line.strip()]
            filtered_lines = []
            
            for line in lines:
                # Skip lines that look like metadata
                if any([
                    re.match(r'^[^a-zA-Z]*$', line),           # Lines without letters
                    len(line.split()) <= 2,                    # Very short phrases
                    re.match(r'^\s*\d+\s*$', line),           # Just numbers
                    '|' in line,                              # Contains pipe
                    ':' in line,                              # Contains colon
                    line.strip().endswith('.com'),            # URLs
                    line.lower().startswith(('image', 'photo', 'credit', 'source', 'website')),
                ]):
                    continue
                filtered_lines.append(line)
            
            # Join the clean lines
            text = '. '.join(filtered_lines)
            
            # Final cleanup
            text = text.strip()
            text = re.sub(r'\s+', ' ', text)  # Normalize spaces
            text = re.sub(r'\.+', '.', text)  # Normalize periods
            text = re.sub(r'\s+\.', '.', text)  # Fix spaces before periods
            
            return text.strip()

    def detect_language(self, text):
        """Detect language using langdetect library"""
        try:
            if not text or len(text.strip()) < 10:
                return False
            language = detect(text)
            print(f"Detected language: {language}")
            return language == 'es'
        except LangDetectException as e:
            print(f"Language detection failed: {str(e)}, assuming English")
            return False

    def translate_with_retry(self, text, src='EN', dest='ES', max_retries=3):
        """Attempt translation with retries and proper error handling"""
        if not text:
            print("Empty text provided for translation")
            return None
            
        print(f"Translating text: {text[:100]}...")
        
        for attempt in range(max_retries):
            try:
                result = self.translator.translate_text(
                    text,
                    source_lang=src,
                    target_lang=dest
                )
                
                if result:
                    translated_text = str(result)
                    print(f"Translation successful: {translated_text[:100]}...")
                    return translated_text
                    
                print(f"Translation attempt {attempt + 1} failed: No translation returned")
                
            except Exception as e:
                print(f"Translation attempt {attempt + 1} failed with error: {str(e)}")
                if attempt < max_retries - 1:
                    print("Waiting before retry...")
                    time.sleep(2)
        
        return None

    def extract_key_point(self, summary):
        """Extract key point from summary"""
        summary = self.clean_text(summary)
        sentences = [s.strip() for s in summary.split('.') if s.strip()]
        key_sentences = [s for s in sentences[:2] if len(s.split()) > 3]
        return '. '.join(key_sentences) + '.' if key_sentences else summary[:100] + "..."

    def create_post(self, entry):
        """Create a social media post"""
        try:
            if self.is_duplicate(entry.title):
                print("Duplicate post found, skipping...")
                return None
            
            # Clean texts first
            clean_title = self.clean_text(entry.title)
            clean_summary = self.extract_key_point(entry.summary)

            # Check if content is promotional
            is_promo, reason = self.is_promotional_content(clean_title, clean_summary)
            if is_promo:
                print(f"Skipping promotional content: {reason}")
                return None

            print(f"\nProcessing article: {clean_title[:100]}...")
            
            # Detect language
            is_spanish = self.detect_language(f"{clean_title}. {clean_summary}")
            
            if is_spanish:
                headline = clean_title
                key_point = clean_summary
                print("Spanish content, no translation needed")
            else:
                print("English content detected, translating...")
                headline = self.translate_with_retry(clean_title)
                if not headline:
                    return None

                key_point = self.translate_with_retry(clean_summary)
                if not key_point:
                    return None

            # Create post
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
            
            if entries_tried >= 15:  # Increased limit since we're filtering more
                print("Tried maximum number of entries, stopping")
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