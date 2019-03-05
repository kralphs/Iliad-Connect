document.getElementById("toggleClio").parentNode.addEventListener("click", toggleClio);
document.getElementById("toggleEmail").parentNode.addEventListener("click", toggleEmail);
document.getElementById("toggleScanning").parentNode.addEventListener("click", toggleScanning);

function toggleClio(e) {
    if(document.getElementById("toggleClio").checked){
        logoutClio().then((xhr)=> {alert("Successful")}, (xhr) => {e.stopPropagation()})
    } else {
        window.location.assign("/auth/clio/login");
    }
};

function toggleEmail(e) {
    if(document.getElementById("toggleEmail").checked){
//        logoutClio()
        if (Math.random() < 0.5){
            alert("Logout Successful")
        } else {
            alert("Logout Failed")
            e.stopPropagation()
        }
    } else {
//        window.location.assign("/auth/google/login");
        alert("Redirect to Login")
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

