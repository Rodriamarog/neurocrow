import React from 'react';
import './Login.css';

function Login() {
  const handleFacebookLogin = () => {
    window.FB.login(function(response) {
      if (response.authResponse) {
        console.log('Successfully logged in:', response);
        // Handle successful login
      } else {
        console.log('User cancelled login or did not fully authorize.');
      }
    }, {
      scope: [
        // Facebook Pages
        'pages_show_list',
        'pages_messaging',
        'pages_read_engagement',
        'pages_manage_metadata',
        'business_management',
        
        // Instagram
        'instagram_basic',
        'instagram_manage_messages',
        'instagram_business_manage_messages',
        
        // WhatsApp
        'whatsapp_business_management',
        'whatsapp_business_messaging'
      ].join(',')
    });
  };

  return (
    <div className="login-container">
      <div className="login-box">
        <h1>Conecta tus cuentas</h1>
        <p>Para comenzar, necesitamos acceso a tus cuentas de redes sociales</p>
        <button onClick={handleFacebookLogin} className="facebook-login-btn">
          <i className="fab fa-facebook"></i> Continuar con Facebook
        </button>
      </div>
    </div>
  );
}

export default Login;