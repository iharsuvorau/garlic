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
    const collectionsOfItems = [].concat(questions, positives, negatives);

    const sessionItemElements = document.getElementsByClassName("session-item-element");

    const notifications = document.getElementById("notifications");

    for (const collection of collectionsOfItems) {
        for (const item of collection) {
            item.addEventListener("click", (event) => {
                // const sessionID = event.target.dataset.sessionid;
                const itemID = event.target.dataset.itemid;
                // const itemType = event.target.dataset.type;

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
                        message = data.message;
                        markSessionItemActive(itemID, sessionItemElements);
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

function markSessionItemActive(itemID, items) {
    for (const item of items) {
        item.classList.remove("active");
    }

    const curItem = document.getElementById(itemID);
    curItem.classList.add("active");
    curItem.classList.add("visited");

    console.log(curItem);
}