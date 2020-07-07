window.addEventListener('DOMContentLoaded', (event) => {
    const sessionList = document.getElementById("sessionList");

    if (sessionList) {
        sessionList.addEventListener("change", (event) => {
            let val = event.target.value;
            document.location.pathname = unescape("/sessions/" + val);
        })
    }

    const questions = document.getElementsByClassName("question");
    const positives = document.getElementsByClassName("positive-answer");
    const negatives = document.getElementsByClassName("negative-answer");

    const notifications = document.getElementById("notifications");
    const sessionItems = document.getElementsByClassName("session-item");

    for (const collection of [].concat(questions, positives, negatives)) {
        for (const item of collection) {
            item.addEventListener("click", (event) => {
                const sessionID = parseInt(event.target.dataset.sessionid);
                const itemID = parseInt(event.target.dataset.itemid);
                const itemType = event.target.dataset.type;

                // requesting the JSON API
                fetch("/pepper/send_command", {
                    method: "POST",
                    headers: {"Content-Type": "application/json"},
                    body: JSON.stringify({
                        "session_id": sessionID,
                        "item_type": itemType,
                        "item_id": itemID,
                    })
                }).then(response => {
                    console.log("got response:", response.status, response.statusText);
                    return response.json();
                }).then(data => {
                    let message = "error";
                    let notificationClass = "message";
                    if (data.message && data.message.length > 0) {
                        message = data.message;
                        markSessionItemActive(itemID, sessionItems);
                    } else if (data.error && data.error.length > 0) {
                        message = data.error;
                        notificationClass = "error";
                    }

                    // creating a notification
                    const notification = document.createElement("div");
                    notification.classList.add("notification-item");
                    notification.classList.add(notificationClass);
                    notification.innerText = message;
                    notifications.appendChild(notification);

                    // removing the notification after some time
                    const timeoutID = window.setTimeout(() => {
                        window.clearTimeout(timeoutID);
                        console.log("timer stopped");
                        notifications.removeChild(notification);
                    }, 1500);
                }).catch(error => {
                    console.log("error:", error)
                })
            })
        }
    }
});

function debug(label, content) {
    console.log(label + ":", content);
}

function markSessionItemActive(itemID, items) {
    const curItem = document.getElementById("session-item-" + String(itemID));
    for (const item of items) {
        if (curItem.dataset.itemid != item.dataset.itemid) {
            item.classList.remove("active");
        } else {
            item.classList.add("active");
        }
    }
}