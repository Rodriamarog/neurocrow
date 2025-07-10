package views

import (
	"admin-dashboard/models"
	"admin-dashboard/pkg/auth"
)

func ToMessageViews(messages []models.Message) []MessageView {
	messageViews := make([]MessageView, len(messages))
	for i, msg := range messages {
		var userName string
		if msg.SocialUserName != nil {
			userName = *msg.SocialUserName
		}

		messageViews[i] = MessageView{
			ID:        msg.ID,
			Content:   msg.Content,
			Timestamp: msg.Timestamp,
			Sender: UserView{
				Name: userName,
			},
			Thread: ThreadView{
				ID:         msg.ThreadID,
				ProfilePic: msg.ProfilePictureURL,
			},
		}
	}
	return messageViews
}

func ToUserView(user *auth.User) UserView {
	return UserView{
		ID:       user.ID,
		ClientID: user.ClientID,
		Role:     user.Role,
		Name:     user.ClientID,
	}
}
