// Login.js
import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';

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
    <div className="min-h-screen bg-slate-50 flex items-center justify-center p-4">
      <div className="w-full max-w-md bg-white border border-slate-200 p-8 space-y-6">
        <div className="text-center">
          <h1 className="text-2xl font-bold text-slate-900">Conecta tus cuentas</h1>
        </div>
        {isVerifying ? (
          <div className="space-y-6">
            <div className="text-center">
              <div className="w-12 h-12 mx-auto mb-4 text-slate-600">
                <i className="fas fa-spinner fa-spin text-2xl"></i>
              </div>
              <h2 className="text-xl font-semibold text-slate-900 mb-2">Verificación pendiente</h2>
              <p className="text-slate-600">Por favor, aprueba el inicio de sesión en tu aplicación de {authType === 'facebook' ? 'Facebook' : 'Instagram'}.</p>
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
              className="w-full px-4 py-2 text-sm font-medium text-slate-700 bg-white border border-slate-300 hover:bg-slate-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 transition-colors"
            >
              Cancelar
            </button>
          </div>
        ) : (
          <div className="space-y-6">
            <div className="text-center">
              <p className="text-slate-600">Para comenzar, conecta tus cuentas de redes sociales</p>
            </div>
            
            <div className="space-y-3">
              <button 
                onClick={handleFacebookLogin} 
                className="w-full flex items-center justify-center gap-3 px-4 py-3 bg-blue-600 hover:bg-blue-700 text-white font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
              >
                <i className="fab fa-facebook text-lg"></i>
                Conectar Facebook
              </button>

              <button 
                onClick={handleInstagramLogin} 
                className="w-full flex items-center justify-center gap-3 px-4 py-3 bg-gradient-to-r from-purple-500 to-pink-500 hover:from-purple-600 hover:to-pink-600 text-white font-medium transition-all focus:outline-none focus:ring-2 focus:ring-purple-500 focus:ring-offset-2"
              >
                <i className="fab fa-instagram text-lg"></i>
                Conectar Instagram
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

export default Login;