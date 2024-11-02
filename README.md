# NeuroCrow 🤖

NeuroCrow is an automated AI news curator and social media content generator focused on artificial intelligence and technology news for Spanish-speaking audiences. It transforms English AI news into engaging Spanish social media content.

## Features

- 🔄 Automated news scraping from top tech sources
- 🌐 English to Spanish translation
- 📱 Social media-friendly post generation
- #️⃣ Smart hashtag integration
- 📊 Various post templates for engagement
- ⏱ Scheduling capabilities

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

## Usage

Basic usage example:
```python
from src.content_curator import AIContentCurator

curator = AIContentCurator()
posts = curator.generate_posts(num_posts=3)

for post in posts:
    print(post['post_content'])
```

## Project Structure

```
neurocrow/
│
├── src/
│   ├── __init__.py
│   ├── content_curator.py
│   └── utils.py
├── config/
│   └── settings.py
├── tests/
│   └── __init__.py
├── venv/
├── .gitignore
├── requirements.txt
└── README.md
```

## Dependencies

- feedparser==6.0.10
- googletrans==3.1.0a0
- python-dotenv==1.0.0
- requests==2.31.0
- schedule==1.2.1

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- Built with Python 3.x
- Uses Google Translate API for translations
- RSS feeds from major tech news sources

## Contact

Rodrigo Amaro - rodriamarog@gmail.com

Project Link: [https://github.com/Rodriamarog/neurocrow](https://github.com/Rodriamarog/neurocrow)

---

Made with ❤️ by Rodrigo Amaro