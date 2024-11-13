import json
import feedparser
from dotenv import load_dotenv
import deepl
import os
import re
from bs4 import BeautifulSoup
import html
from langdetect import detect, LangDetectException
import requests
import urllib.request
import socket
import time
import random

class NewsPostGenerator:
    def __init__(self):
        # Load environment variables
        load_dotenv()
        
        # Initialize DeepL translator
        auth_key = os.getenv('DEEPL_API_KEY')
        if not auth_key:
            raise ValueError("DEEPL_API_KEY not found in environment variables")
        self.translator = deepl.Translator(auth_key)

        # Set timeouts for feedparser
        socket.setdefaulttimeout(10)  # 10 seconds timeout
        
        self.feeds = [
            # Major Tech Publications (Working)
            'https://techcrunch.com/category/artificial-intelligence/feed/',
            'https://venturebeat.com/category/ai/feed/',
            'https://www.artificialintelligence-news.com/feed/',
            'https://www.zdnet.com/topic/artificial-intelligence/rss.xml',
            'https://www.theverge.com/ai-artificial-intelligence/rss/index.xml',
            'https://www.marktechpost.com/feed/',
            'https://www.unite.ai/feed/',
            'https://wwwhatsnew.com/category/inteligencia-artificial/feed/',
            'https://hipertextual.com/tag/inteligencia-artificial/feed',
            'https://blogs.nvidia.com/feed/',
            'https://machinelearning.apple.com/rss.xml',
            'https://www.technologyreview.com/topic/artificial-intelligence/feed',
            'https://blog.deeplearning.ai/rss.xml',
            'https://medium.com/feed/tag/artificial-intelligence',
            'https://planetachatbot.com/feed',
            'https://iabot.org/feed/'
        ]

    def fetch_feed(self, feed_url):
        """Fetch feed with timeout and error handling"""
        try:
            print(f"Fetching feed: {feed_url}")
            # Use requests to fetch with timeout
            response = requests.get(feed_url, timeout=10)
            if response.status_code == 200:
                return feedparser.parse(response.content)
            else:
                print(f"Failed to fetch {feed_url}: Status {response.status_code}")
                return None
        except requests.Timeout:
            print(f"Timeout fetching {feed_url}")
            return None
        except Exception as e:
            print(f"Error fetching {feed_url}: {str(e)}")
            return None

    def post_to_facebook(self, post_content):
        """Post content to Facebook with link preview"""
        try:
            token = os.getenv('FACEBOOK_PAGE_TOKEN')
            page_id = os.getenv('FACEBOOK_PAGE_ID')
            
            if not token or not page_id:
                print("Missing Facebook credentials!")
                return False
            
            print("Posting to Facebook...")
            url = f"https://graph.facebook.com/v18.0/{page_id}/feed"
            
            # Create post with link preview
            data = {
                'message': post_content['content'],
                'link': post_content['link'],  # This triggers link preview
                'access_token': token
            }
            
            response = requests.post(url, data=data, timeout=10)
            success = response.status_code == 200
            
            if success:
                print("Successfully posted to Facebook!")
                post_id = response.json().get('id')
                print(f"Post ID: {post_id}")
            else:
                print(f"Failed to post: {response.text}")
                
            return success
            
        except Exception as e:
            print(f"Error posting to Facebook: {str(e)}")
            return False

    def clean_text(self, text):
        """Clean text from HTML and unwanted content"""
        # Initial cleaning
        text = html.unescape(text)
        soup = BeautifulSoup(text, 'html.parser')
        text = soup.get_text()
        
        patterns_to_remove = [
            # Brand-specific endings
            r'(?i)Y la NVIDIA.*$',
            r'Y (?:el|la|los|las)\s+\w+\s+(?:leer|ver).*?$',
            r'(?i)Y (?:el|la|los|las)\s+[A-Za-z]+\s*$',
            
            # Article endings and calls-to-action
            r'(?i)leer (?:el )?artículo\.?$',
            r'(?i)leer más\.?$',
            r'(?i)leer nota\.?$',
            r'(?i)ver más\.?$',
            r'(?i)más información\.?$',
            r'(?i)continuar leyendo\.?$',
            r'Los (?:usuarios|jugadores).*?(?:leer|ver|más).*?$',
            
            # Partial endings
            r'Los que .*?$',
            r'Para (?:leer|ver|más).*?$',
            r'Si quieres .*?$',
            r'Puedes .*?$',
            
            # Source attributions
            r'The post.*?appeared first on.*?$',
            r'(?i)originally published(?:.*?)$',
            r'(?i)originally posted(?:.*?)$',
            r'(?i)published by(?:.*?)$',
            r'(?i)posted by(?:.*?)$',
            r'(?i)written by(?:.*?)$',
            
            # Footer elements
            r'(?i)read more at(?:.*?)$',
            r'(?i)read the full(?:.*?)$',
            r'(?i)continua leyendo(?:.*?)$',
            r'(?i)continúa leyendo(?:.*?)$',
            r'(?i)seguir leyendo:.*$',
            r'(?i)seguir leyendo.*$',
            r'\d{4}\s+TechCrunch\.\s+Todos los derechos reservados\..*$',
            r'(?i)solo para uso personal\.?',
            r'(?i)leer más:.*$',
            
            # Source citations
            r'Source:.*?$',
            r'Fuente:.*?$',
            r'Via:.*?$',
            r'Vía:.*?$',
            r'Un equipo de investigadores de.*?(?:appeared|posted|published).*?$',
            r'.*?(?:appeared|posted|published) (?:first )?on [A-Za-z0-9\.]+ ?$',
            
            # Image and media references
            r'(?i)ilustración de.*?(?=\n|$)',
            r'(?i)illustration by.*?(?=\n|$)',
            r'(?i)foto de.*?(?=\n|$)',
            r'(?i)photo by.*?(?=\n|$)',
            r'(?i)imagen.*?\/.*?(?=\n|$)',
            r'.*?\/ The Verge',
            
            # URLs and technical elements
            r'http[s]?://\S+',
            r'\.{3,}',
            r'\[…\]',
            r'\[\.\.\.\]',
        ]
        
        # Apply all cleaning patterns
        for pattern in patterns_to_remove:
            text = re.sub(pattern, '', text)
        
        # Clean up whitespace
        text = re.sub(r'\s+', ' ', text)
        text = text.strip()
        
        # Handle sentences
        sentences = text.split('.')
        clean_sentences = []
        
        for sentence in sentences:
            sentence = sentence.strip()
            # Skip if sentence is too short or looks like a partial ending
            if len(sentence.split()) <= 3:
                continue
            if any(phrase in sentence.lower() for phrase in 
                ['leer', 'ver', 'más', 'continuar', 'click', 'visita', 'descubre']):
                continue
            if sentence:
                clean_sentences.append(sentence)
        
        # Rebuild text
        cleaned_text = '. '.join(clean_sentences)
        
        # Ensure proper ending
        if cleaned_text and not cleaned_text[-1] in '.!?':
            cleaned_text += '.'
            
        return cleaned_text.strip()

    def is_low_quality(self, title, summary):
        """Check if content is low quality with refined criteria"""
        # Only skip clearly low-quality content
        definite_low_quality = [
            r'^\d+\s+[A-Za-z\s]+$',           # Just numbered lists
            r'noticias mensuales',             # Monthly roundups
            r'noticias semanales',             # Weekly roundups
            'the post first appeared',         # Metadata text
            'click aquí',                      # Navigation text
        ]
        
        text = (title + ' ' + summary).lower()
        
        # Check for definite low-quality markers
        if any(re.search(pattern, text, re.IGNORECASE) for pattern in definite_low_quality):
            return True
        
        # Check minimum content requirements
        clean_summary = self.clean_text(summary)
        words = clean_summary.split()
        if len(words) < 15:  # Very short content
            return True
        
        # Check for excessive formatting
        if summary.count('\n') > 10:  # Too many line breaks
            return True
            
        # Check for interview/review markers only in title
        interview_markers = ['interview', 'review', 'guide', 'tutorial']
        if any(marker in title.lower() for marker in interview_markers):
            return True
            
        return False

    def truncate_at_sentence(self, text, max_length=800):
        """Ensure text ends with a complete sentence and makes logical sense"""
        if len(text) <= max_length:
            return text

        # Split into sentences more carefully
        # This handles more Spanish punctuation and avoids cutting at abbreviations
        sentences = []
        current = []
        
        # Split text into words
        words = text.split()
        
        for word in words:
            current.append(word)
            # Check for sentence endings but avoid splitting on common abbreviations
            if (word.endswith(('.', '!', '?')) and 
                not any(word.lower().startswith(abbr) for abbr in ['sr.', 'sra.', 'dr.', 'dra.', 'prof.', 'etc.', 'ej.', 'vs.'])):
                sentences.append(' '.join(current))
                current = []
        
        # Add any remaining words as the last sentence
        if current:
            sentences.append(' '.join(current))

        # Build result while checking length
        result = []
        current_length = 0
        
        for sentence in sentences:
            if current_length + len(sentence) + 1 <= max_length:
                result.append(sentence)
                current_length += len(sentence) + 1
            else:
                break
        
        # If we have no complete sentences, just take the first one
        if not result and sentences:
            return sentences[0]
        
        final_text = ' '.join(result)
        
        # Clean up any trailing partial words or symbols
        final_text = re.sub(r'[^\w\s.!?]$', '', final_text)  # Remove trailing symbols
        final_text = re.sub(r'\s+[^\s]+©$', '.', final_text)  # Remove partial copyright symbols
        final_text = re.sub(r'\s+$', '', final_text)  # Remove trailing spaces
        
        # Ensure proper ending
        if not final_text[-1] in '.!?':
            final_text += '.'
            
        return final_text

    def format_post(self, title, summary, link):
        """Format post content with emojis and double line breaks after sentences"""
        # List of emoji pairs for titles (eye-catching but professional)
        title_emoji_pairs = [
            ('🔥', '🔥'),  # Fire
            ('⚡', '⚡'),  # Lightning bolt
            ('🤖', '💡'),  # Robot + Lightbulb
            ('🌟', '✨'),  # Star + Sparkles
            ('💫', '🚀'),  # Stars + Rocket
            ('🎯', '💡'),  # Target + Lightbulb
            ('🔮', '✨'),  # Crystal ball + Sparkles
            ('💡', '🎯'),  # Lightbulb + Target
            ('🌐', '⚡'),  # Globe + Lightning
            ('🎮', '🤖'),  # For gaming/tech news
        ]
        
        # Randomly select an emoji pair
        start_emoji, end_emoji = random.choice(title_emoji_pairs)
        
        # Clean text first
        cleaned_summary = self.clean_text(summary)
        
        # Ensure summary doesn't end abruptly
        truncated_summary = self.truncate_at_sentence(cleaned_summary)
        
        # Double-check the ending
        if not truncated_summary[-1] in '.!?':
            truncated_summary += '.'
        
        # Split summary into sentences and add double line breaks
        sentences = re.split(r'([.!?])\s+', truncated_summary)
        
        # Rejoin sentences with double line breaks, preserving punctuation
        formatted_summary = ''
        for i in range(0, len(sentences)-1, 2):
            sentence = sentences[i]
            punctuation = sentences[i+1] if i+1 < len(sentences) else '.'
            formatted_summary += f"{sentence}{punctuation}\n\n"
        
        # Add any remaining text
        if len(sentences) % 2:
            formatted_summary += sentences[-1]
        
        # Format complete post with emojis
        post = f"{start_emoji} {title} {end_emoji}\n\n{formatted_summary.strip()}\n\n#IA #Tech #Innovación #NeuroCrow #Tijuana"
        
        return {
            'content': post,
            'link': link
        }

    def is_promotional(self, text):
        """Check if content is promotional"""
        promo_words = ['deal', 'sale', 'discount', 'offer', 'price', 'shop', 
                      'buy', 'black friday', 'cyber monday', 'review']
        text_lower = text.lower()
        return any(word in text_lower for word in promo_words)

    def score_article(self, title, summary, published_date=None):
            """Score an article based on various factors"""
            score = 0
            
            # Length scoring (prefer medium-length content)
            title_length = len(title.split())
            if 5 <= title_length <= 15:
                score += 10
            elif 3 <= title_length <= 20:
                score += 5
                
            # Content relevance scoring (key AI terms)
            ai_keywords = [
                'ia', 'ai', 'inteligencia artificial', 'artificial intelligence',
                'machine learning', 'deep learning', 'neural', 'modelo', 'model',
                'chatgpt', 'gpt', 'llm', 'openai', 'google', 'microsoft',
                'automatización', 'automation', 'robot', 'data'
            ]
            
            content = (title + ' ' + summary).lower()
            keyword_matches = sum(1 for keyword in ai_keywords if keyword in content)
            score += keyword_matches * 5
            
            # Freshness scoring (if published date available)
            if published_date:
                try:
                    published = datetime.fromtimestamp(time.mktime(published_date))
                    hours_old = (datetime.now() - published).total_seconds() / 3600
                    if hours_old < 24:
                        score += 20
                    elif hours_old < 48:
                        score += 10
                    elif hours_old < 72:
                        score += 5
                except Exception:
                    pass
            
            return score

    def generate_post(self):
        """Generate a single post with quality control and better error handling"""
        candidate_articles = []
        
        for feed_url in self.feeds:
            feed = self.fetch_feed(feed_url)
            if not feed:
                continue
                
            print(f"Processing entries from {feed_url}")
            
            for entry in feed.entries:
                try:
                    # Clean and check content
                    title = self.clean_text(entry.title)
                    summary = self.clean_text(entry.summary)
                    
                    if not title or not summary:
                        print(f"Skipping entry: Empty title or summary")
                        continue
                        
                    # Skip promotional and low-quality content
                    if self.is_promotional(title) or self.is_promotional(summary):
                        print(f"Skipping promotional content: {title}")
                        continue
                        
                    if self.is_low_quality(title, summary):
                        print(f"Skipping low-quality content: {title}")
                        continue

                    # Add to candidates
                    try:
                        is_spanish = detect(title + " " + summary) == 'es'
                        candidate_articles.append({
                            'title': title,
                            'summary': summary,
                            'link': entry.link,
                            'is_spanish': is_spanish
                        })
                    except LangDetectException as e:
                        print(f"Language detection failed for: {title}")
                        continue
                    
                except Exception as e:
                    print(f"Error processing entry: {str(e)}")
                    continue
        
        if not candidate_articles:
            print("No suitable articles found")
            return None
            
        # Randomly select from VALID articles
        selected_article = random.choice(candidate_articles)
        print(f"\nSelected article: {selected_article['title']}")
        print(f"Quality check: Passed ✓")
        print(f"Language: {'Spanish' if selected_article['is_spanish'] else 'English'}")
        
        # Translation attempt with detailed error handling
        try:
            if not selected_article['is_spanish']:
                print("\nTranslating content...")
                try:
                    translated_title = self.translator.translate_text(
                        selected_article['title'], 
                        target_lang='ES'
                    )
                    print("✓ Title translated successfully")
                    
                    translated_summary = self.translator.translate_text(
                        selected_article['summary'], 
                        target_lang='ES'
                    )
                    print("✓ Summary translated successfully")
                    
                    title = str(translated_title)
                    summary = str(translated_summary)
                except Exception as e:
                    print(f"Translation failed: {str(e)}")
                    return None
            else:
                title = selected_article['title']
                summary = selected_article['summary']
                print("No translation needed - content already in Spanish")

            # Format post
            print("\nFormatting post...")
            post = self.format_post(title, summary, selected_article['link'])
            print("✓ Post formatted successfully")
            
            return post
            
        except Exception as e:
            print(f"\nError in post generation: {str(e)}")
            import traceback
            print(traceback.format_exc())
            return None

def lambda_handler(event, context):
    """AWS Lambda handler"""
    try:
        generator = NewsPostGenerator()
        post = generator.generate_post()
        
        if post and generator.post_to_facebook(post):
            return {
                'statusCode': 200,
                'body': json.dumps('Posted successfully!')
            }
        else:
            return {
                'statusCode': 500,
                'body': json.dumps('Failed to generate or post content')
            }
            
    except Exception as e:
        return {
            'statusCode': 500,
            'body': json.dumps(f'Error: {str(e)}')
        }

# For local testing
if __name__ == "__main__":
    print("Starting news post generator...")
    generator = NewsPostGenerator()
    
    print("\nGenerating post...")
    post = generator.generate_post()
    
    if post:
        print("\nGenerated post:")
        print("-" * 50)
        print(post['content'])
        print("-" * 50)
        
        should_post = input("\nPost to Facebook? (y/n): ")
        if should_post.lower() == 'y':
            if generator.post_to_facebook(post):
                print("Posted successfully!")
            else:
                print("Failed to post")
    else:
        print("\nFailed to generate post!")