document.getElementById("toggleClio").parentNode.addEventListener("click", toggleClio);
document.getElementById("toggleEmail").parentNode.addEventListener("click", toggleEmail);
document.getElementById("toggleScanning").parentNode.addEventListener("click", toggleScanning);

function toggleClio(e) {
    if(document.getElementById("toggleClio").checked){
        logoutClio().then((xhr)=> {}, (xhr) => {e.stopPropagation()})
    } else {
        window.location.assign("/auth/clio/login");
    }
};

function toggleEmail(e) {
    // TODO: Add way to inform which email provider is used id:413
    var provider = "google"
    if(document.getElementById("toggleEmail").checked){
        logoutEmail(provider).then((xhr)=> {}, (xhr) => {e.stopPropagation()})
    } else {
        window.location.assign("/auth/" + provider + "/login");
    }
};

function toggleScanning(e) {
    if(document.getElementById("toggleScanning").checked){
//        logoutClio().then((xhr)=> {}, (xhr) => {e.stopPropagation()})
    } else {
//        window.location.assign("/auth/google/login");
        alert("Redirect to Login")
    }
};


function logoutClio(){
    var xhr = new XMLHttpRequest;

    return new Promise((resolve, reject) => {
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
          xhr.open('GET', '/auth/clio/logout', true);
          xhr.send(null);      
    });
}

function logoutEmail(provider){
    var xhr = new XMLHttpRequest;

    return new Promise((resolve, reject) => {
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
          xhr.open('GET', '/auth/' + provider + '/logout', true);
          xhr.send(null);      
    });
}
