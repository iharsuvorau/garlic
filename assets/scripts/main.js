window.addEventListener('DOMContentLoaded', (event) => {
    const sessionList = document.getElementById("sessionList");

    if (sessionList) {
        sessionList.addEventListener("change", (event) => {
            let val = event.target.value;
            document.location.pathname = unescape("/sessions/" + val);
        })
    }

    const sessionItemElements = document.getElementsByClassName("session-item-element");
    const notifications = document.getElementById("notifications");

    // creating an audio environment
    const AudioContext = window.AudioContext || window.webkitAudioContext;  // for cross browser
    let audioCtx, audioElement;
    audioCtx = new AudioContext();

    for (const item of sessionItemElements) {
        item.addEventListener("click", (event) => {
            // preparing request data
            const itemID = event.target.dataset.itemid;
            const requestData = {
                "item_id": itemID,
            };
            console.log("request:", requestData);

            // requesting the JSON API
            fetch("/pepper/send_command", {
                method: "POST",
                headers: {"Content-Type": "application/json"},
                body: JSON.stringify(requestData)
            }).then(response => {
                return response.json();
            }).then(data => {
                console.log(data);
                let message = "error";
                let notificationClass = "message";
                if (data.message && data.message.length > 0) {
                    // message for a notification in UI
                    message = data.message;

                    // marking as "visited" and "active" in UI
                    markSessionItemActive(itemID, sessionItemElements);

                    // playing an audio
                    audioElement = document.getElementById("audio-" + itemID);
                    if (audioElement) {
                        try {
                            audioElement.play()
                        } catch (err) {
                            console.error(err);
                        }
                    }
                } else if (data.error && data.error.length > 0) {
                    message = data.error;
                    notificationClass = "error";
                }

                showNotifications(notificationClass, message, notifications)
            }).catch(error => {
                console.log("error:", error)
            })
        })
    }
});

function debug(label, content) {
    console.log(label + ":", content);
}

function markSessionItemActive(itemID, items) {
    for (const item of items) {
        item.classList.remove("active");
    }

    const curItem = document.getElementById(itemID);
    curItem.classList.add("active");
    curItem.classList.add("visited");

    console.log(curItem);
}

function showNotifications(label, message, notifications) {
    const notification = document.createElement("div");
    notification.classList.add("notification-item");
    notification.classList.add(label);
    notification.innerText = message;
    notifications.appendChild(notification);

    // removing the notification after some time
    const timeoutID = window.setTimeout(() => {
        window.clearTimeout(timeoutID);
        notifications.removeChild(notification);
    }, 1500);
}