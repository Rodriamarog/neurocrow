import React from 'react';
import { useNavigate } from 'react-router-dom';
import './Login.css';

function Login() {
  const navigate = useNavigate();

  const handleFacebookLogin = () => {
    window.FB.login(function(response) {
      if (response.authResponse) {
        console.log('Successfully logged in:', response);
        // Navigate to success page after successful login
        navigate('/success');
      } else {
        console.log('User cancelled login or did not fully authorize.');
      }
    }, {
      scope: [
        // Facebook Pages
        'pages_show_list',
        'pages_messaging',
        
        // Instagram
        'instagram_basic',
        'instagram_manage_messages',
        
        // WhatsApp
        'whatsapp_business_messaging'
      ].join(',')
    });
  };

  return (
    <div className="login-container">
      <div className="login-box">
        <h1>Conecta tus cuentas</h1>
        <p>Para comenzar, conecta tus cuentas de redes sociales a nuestra app</p>
        <button onClick={handleFacebookLogin} className="facebook-login-btn">
          <i className="fab fa-facebook"></i> Continuar con Facebook
        </button>
      </div>
    </div>
  );
}

export default Login;