import './Home.css';

function Home() {
  return (
    <div className="home">
      <section className="hero">
        <div className="hero-content">
          <h1>Transformamos la Comunicación con Chatbots Inteligentes</h1>
          <p>Integración perfecta con Facebook Messenger, Instagram y WhatsApp</p>
          <div className="video-placeholder">
            {/* Placeholder para el video demo */}
            <div className="video-container">
              <div className="video-mock">Video Demo</div>
            </div>
          </div>
        </div>
      </section>

      <section className="features">
        <h2>Nuestros Servicios</h2>
        <div className="features-grid">
          <div className="feature-card">
            <i className="fab fa-facebook-messenger"></i>
            <h3>Facebook Messenger</h3>
            <p>Chatbots personalizados para tu página de Facebook</p>
          </div>
          <div className="feature-card">
            <i className="fab fa-instagram"></i>
            <h3>Instagram</h3>
            <p>Automatiza tus respuestas en Instagram Direct</p>
          </div>
          <div className="feature-card">
            <i className="fab fa-whatsapp"></i>
            <h3>WhatsApp</h3>
            <p>Integración con WhatsApp Business API</p>
          </div>
        </div>
      </section>

      <section className="cta">
        <div className="cta-content">
          <h2>¿Listo para revolucionar tu atención al cliente?</h2>
          <button className="cta-button">Contáctanos</button>
        </div>
      </section>
    </div>
  );
}

export default Home; 