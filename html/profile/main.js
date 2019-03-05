document.getElementById("toggleClio").parentNode.addEventListener("click", toggleClio);
document.getElementById("toggleEmail").parentNode.addEventListener("click", toggleEmail);
document.getElementById("toggleScanning").parentNode.addEventListener("click", toggleScanning);

function toggleClio(e) {
    if(document.getElementById("toggleClio").checked){
        logoutClio()
        if (Math.random() < 0.5){
            alert("Logout Successful")
        } else {
            alert("Logout Failed")
            e.stopPropagation()
        }
    } else {
//        window.location.assign("/auth/clio/login");
        alert("Redirect to Login")
    }
};

function toggleEmail(e) {
    if(document.getElementById("toggleEmail").checked){
        logoutClio()
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
        logoutClio()
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


function logoutClio(){
    alert("Should Log Out");
}

