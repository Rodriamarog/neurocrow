import React from 'react';
import './Success.css';

function Success() {
  const handleContactClick = () => {
    // Using your existing Facebook page messenger link
    window.open('https://m.me/413548765185533', '_blank');
  };

  return (
    <div className="success-container">
      <div className="success-box">
        <i className="fas fa-check-circle success-icon"></i>
        <h1>¡Conexión Exitosa!</h1>
        <p>Gracias por conectar tus cuentas con Neurocrow. Nos pondremos en contacto contigo pronto para configurar tu chatbot.</p>
        <div className="contact-options">
          <p>Si tienes alguna pregunta, puedes contactarnos por:</p>
          <div className="contact-buttons">
            <button onClick={handleContactClick} className="messenger-btn">
              <i className="fab fa-facebook-messenger"></i> Messenger
            </button>
            <a 
              href="https://wa.me/+16197612314" 
              target="_blank" 
              rel="noopener noreferrer" 
              className="whatsapp-btn"
            >
              <i className="fab fa-whatsapp"></i> WhatsApp
            </a>
          </div>
        </div>
      </div>
    </div>
  );
}

export default Success;