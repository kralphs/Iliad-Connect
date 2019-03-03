      // Initialize Firebase
var config = {
    apiKey: "AIzaSyBDQSrJenljnhUiRce5tUgSRJjRoapXkT0",
    authDomain: "iliad-connect-227218.firebaseapp.com",
    databaseURL: "https://iliad-connect-227218.firebaseio.com",
    projectId: "iliad-connect-227218",
    storageBucket: "iliad-connect-227218.appspot.com",
    messagingSenderId: "502779193562"
};
firebase.initializeApp(config);
firebase.auth().setPersistence(firebase.auth.Auth.Persistence.NONE);
var firestore = firebase.firestore();

function postIdTokenToSessionLogin(csrfToken, idToken) {
    var xhr = new XMLHttpRequest();
    formData = new FormData();

    formData.set("token", idToken);
    formData.set("gorilla.csrf.Token",csrfToken)
    xhr.open('POST','/sessionLogin', false);
    xhr.send(formData);
};

function getSessionCSRFToken() {
  var xhr = new XMLHttpRequest();

  return new Promise(function (resolve, reject) {
    xhr.onload = function () {
      if (xhr.readyState=== 4 && xhr.status === 200) {
        resolve(xhr);
      } else {
        reject({
              status: xhr.status,
              statusText: xhr.statusText
          });
      };
    };
    xhr.open('GET', '/sessionLogin', true);
    xhr.send(null);
    });
  };

  // FirebaseUI config.
  var uiConfig = {
      signInSuccessUrl: '/',
      signInOptions: [
          // Leave the lines as is for the providers you want to offer your users.
          firebase.auth.GoogleAuthProvider.PROVIDER_ID
      ],
      // tosUrl and privacyPolicyUrl accept either url string or a callback
      // function.
      // Terms of service url/callback.
      tosUrl: '<your-tos-url>',
      // Privacy policy url/callback.
      privacyPolicyUrl: function() {
          window.location.assign('<your-privacy-policy-url>');
      }
  };

  initApp = function() {
      firebase.auth().onAuthStateChanged(function(user) {
          if (user) {
              // Get the user's ID token as it is needed to exchange for a session cookie.
              return user.getIdToken().then(idToken => {
                  // Session login endpoint is queried and the session cookie is set.
                  // CSRF protection should be taken into account.
                  // ...  
                  getSessionCSRFToken().then(xhr => {
                    postIdTokenToSessionLogin(xhr.getResponseHeader('X-CSRF-TOKEN'), idToken)
                  }).catch(error => {
                    console.log('Failed to retrieve session token')
                  });
              }).then(() => {
                  // A page redirect would suffice as the persistence is set to NONE.
                  return firebase.auth().signOut();
              }).then(() => {
                  window.location.assign('/profile');
              });
          } else {
              // User is signed out.
          
              // Initialize the FirebaseUI Widget using Firebase.
              var ui = new firebaseui.auth.AuthUI(firebase.auth());
              // The start method will wait until the DOM is loaded.
              ui.start('#firebaseui-auth-container', uiConfig);
          }
      }, function(error) {
          console.log(error);
      });
  };

  window.addEventListener('load', function() {
      initApp()
  });

// Code for email gathering

document.getElementById("top-submit").addEventListener("click", submitTop);
document.getElementById("bottom-submit").addEventListener("click", submitBottom);

function submitTop(e){
    e.preventDefault();
    var emailField = document.getElementById("top-email")

    saveEmail(emailField.value);
    emailField.value = "";
};

function submitBottom(e){
    e.preventDefault();
    var emailField = document.getElementById("bottom-email")

    saveEmail(emailField.value);
    emailField.value = "";
};

function saveEmail(email){
    firestore.collection("leadEmails").add({
        email: email
    })
    .then(function(docRef) {
        console.log("Email added");
    })
    .catch(function(error) {
        console.error("Error adding document: ", error);
    });
}