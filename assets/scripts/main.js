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

    // collecting all questions and answers to be able to mark them as active on clicking
    const questionItems = document.getElementsByClassName("question");
    const positiveItems = document.getElementsByClassName("positive-answer");
    const negativeItems = document.getElementsByClassName("negative-answer");
    const collectionsOfItems = [].concat(questionItems, positiveItems, negativeItems);

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
                    return response.json();
                }).then(data => {
                    let message = "error";
                    let notificationClass = "message";
                    if (data.message && data.message.length > 0) {
                        message = data.message;
                        markSessionItemActive(sessionID, itemID, itemType, collectionsOfItems);
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

function markSessionItemActive(sessionID, itemID, itemType, collections) {
    const curItem = document.getElementById(itemType + "-" + String(sessionID) + "." + String(itemID));

    for (const collection of collections) {
        for (const item of collection) {
            item.classList.remove("active");
        }
    }

    curItem.classList.add("active");
    curItem.classList.add("visited");
}