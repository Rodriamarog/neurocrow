// Login.js
import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import './Login.css';

function Login() {
  const navigate = useNavigate();
  const [isVerifying, setIsVerifying] = useState(false);
  const [pollInterval, setPollInterval] = useState(null);
  const [authType, setAuthType] = useState(null); // 'facebook' or 'instagram'

  useEffect(() => {
    return () => {
      if (pollInterval) {
        clearInterval(pollInterval);
      }
    };
  }, [pollInterval]);

  const startPolling = () => {
    let attempts = 0;
    const maxAttempts = 60; // 2 minutes maximum polling time
    
    const interval = setInterval(() => {
      window.FB.getLoginStatus((response) => {
        attempts++;
        
        if (response.status === 'connected') {
          clearInterval(interval);
          setIsVerifying(false);
          const token = response.authResponse.accessToken;
          navigate('/success', { state: { accessToken: token } });
        } else if (attempts >= maxAttempts) {
          clearInterval(interval);
          setIsVerifying(false);
        }
      }, true);
    }, 2000);

    setPollInterval(interval);
  };

  const handleFacebookLogin = () => {
    setAuthType('facebook');
    
    const scopeArray = [
      'pages_show_list',
      'pages_manage_metadata',
      'pages_messaging',
      'pages_read_engagement',
      'public_profile',
      'business_management'
    ];
    
    const scope = scopeArray.join(',');
    
    console.log('🔍 Requesting Facebook-only permissions:', scope);

    window.FB.login(function(response) {
      console.log('Facebook login response:', response);
      
      if (response.status === 'connected') {
        const token = response.authResponse.accessToken;
        console.log('Successfully logged in with Facebook token');
        navigate('/success', { state: { accessToken: token } });
      } else if (response.status === 'not_authorized') {
        console.log('Awaiting Facebook device verification...');
        setIsVerifying(true);
        startPolling();
      } else {
        console.log('User cancelled Facebook login or did not fully authorize.');
        setIsVerifying(false);
        setAuthType(null);
      }
    }, {
      scope: scope,
      auth_type: 'rerequest'
    });
  };

  const handleInstagramLogin = () => {
    setAuthType('instagram');
    
    const instagramAppId = '1087630639166741'; // Instagram App ID
    const redirectUri = encodeURIComponent(window.location.origin + '/instagram-callback');
    const scope = encodeURIComponent('instagram_business_basic,instagram_business_manage_messages,instagram_business_manage_comments,instagram_business_content_publish,instagram_business_manage_insights');
    
    const instagramAuthUrl = `https://www.instagram.com/oauth/authorize?` +
      `force_reauth=true&` +
      `client_id=${instagramAppId}&` +
      `redirect_uri=${redirectUri}&` +
      `scope=${scope}&` +
      `response_type=code&` +
      `state=instagram_business_auth`;
    
    console.log('🔍 Redirecting to Instagram Business authorization:', instagramAuthUrl);
    
    // Redirect to Instagram Business authorization
    window.location.href = instagramAuthUrl;
  };

  return (
    <div className="login-container">
      <div className="login-box">
        <h1>Conecta tus cuentas</h1>
        {isVerifying ? (
          <>
            <div className="verification-message">
              <h2>Verificación pendiente</h2>
              <p>Por favor, aprueba el inicio de sesión en tu aplicación de {authType === 'facebook' ? 'Facebook' : 'Instagram Business'}.</p>
              <div className="loading-spinner">
                <i className="fas fa-spinner fa-spin"></i>
              </div>
              <button 
                onClick={() => {
                  setIsVerifying(false);
                  setAuthType(null);
                  if (pollInterval) {
                    clearInterval(pollInterval);
                    setPollInterval(null);
                  }
                }} 
                className="cancel-btn"
              >
                Cancelar
              </button>
            </div>
          </>
        ) : (
          <>
            <p>Para comenzar, conecta tus cuentas de redes sociales a nuestra app</p>
            
            <div className="auth-options">
              <div className="auth-option">
                <h3><i className="fab fa-facebook"></i> Facebook Pages</h3>
                <p>Conecta tus páginas de Facebook para gestionar mensajes y automatizar respuestas.</p>
                <ul>
                  <li>✓ Gestión de mensajes de Facebook Messenger</li>
                  <li>✓ Automatización de respuestas</li>
                  <li>✓ Estadísticas de engagement</li>
                </ul>
                <button onClick={handleFacebookLogin} className="facebook-login-btn">
                  <i className="fab fa-facebook"></i> Conectar Facebook Pages
                </button>
              </div>

              <div className="auth-option">
                <h3><i className="fab fa-instagram"></i> Instagram Business</h3>
                <p>Conecta tus cuentas de Instagram Business para gestionar mensajes directos.</p>
                <ul>
                  <li>✓ Gestión de mensajes directos de Instagram</li>
                  <li>✓ Automatización de respuestas</li>
                  <li>✓ Integración con páginas de Facebook</li>
                </ul>
                <div className="instagram-info">
                  <small>
                    <i className="fas fa-info-circle"></i>
                    Requiere cuenta de Instagram Business vinculada a una página de Facebook
                  </small>
                </div>
                <button onClick={handleInstagramLogin} className="instagram-login-btn">
                  <i className="fab fa-instagram"></i> Conectar Instagram Business
                </button>
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

export default Login;