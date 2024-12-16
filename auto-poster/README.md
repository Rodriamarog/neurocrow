# NeuroCrow 🤖

NeuroCrow is an automated content curator focused on AI and technology news. It fetches content from various tech news sources, translates it to Spanish, and generates clean, formatted posts ready for social media distribution.

## Features

- 🔄 Automated news fetching from multiple tech sources
- 🌐 English to Spanish translation
- 🧹 HTML and metadata cleaning
- 📊 Duplicate content detection
- 📜 7-day post history tracking
- #️⃣ Automated hashtag addition

## Installation

1. Clone the repository:
```bash
git clone https://github.com/Rodriamarog/neurocrow.git
cd neurocrow
```

2. Create and activate virtual environment:
```bash
python -m venv venv

# Windows
venv\Scripts\activate

# macOS/Linux
source venv/bin/activate
```

3. Install dependencies:
```bash
pip install -r requirements.txt
```

## Dependencies

- `feedparser`: RSS feed parsing
- `googletrans`: Text translation (English to Spanish)
- `python-dotenv`: Environment variable management
- `requests`: HTTP requests
- `boto3`: AWS SDK (for future AWS Lambda deployment)
- `schedule`: Task scheduling
- `beautifulsoup4`: HTML content cleaning

## Current News Sources

```python
feeds = [
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
```

## Usage

### Local Development

Run the curator script:
```bash
python src/curator.py
```

The script will:
1. Fetch latest articles from all sources
2. Clean and translate content
3. Generate 2 unique posts
4. Save post history to prevent duplicates

Example output:
```
La IA generativa revoluciona el desarrollo de software

Los desarrolladores están adoptando herramientas de IA para automatizar tareas 
repetitivas y mejorar la productividad en el desarrollo de software.

Más información: https://example.com/article

#IA #Tech #Innovación
```

## Post History Management

- Posts are stored in `post_history.json`
- Automatically maintains a 7-day rolling history
- Used for duplicate detection
- Cleaned up automatically during each run

## Future Plans

- [ ] AWS Lambda deployment
- [ ] EventBridge scheduling
- [ ] Social media API integration
- [ ] Enhanced content filtering
- [ ] Custom hashtag strategies
- [ ] Error notification system

## Project Structure

```
neurocrow/
│
├── src/
│   ├── __init__.py
│   └── curator.py
├── venv/
├── .gitignore
├── requirements.txt
└── README.md
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- Uses multiple RSS feeds from leading tech publications
- Translation powered by Google Translate
- Content cleaning powered by BeautifulSoup4

---

Made by [Rodrigo Amaro]