module Application.Models exposing (Session)

import Concourse
import Message.Message as Message
import SideBar.SideBar as SideBar
import UserState exposing (UserState)


type alias Session =
    SideBar.Model
        { userState : UserState
        , hovered : Maybe Message.DomID
        , clusterName : String
        , turbulenceImgSrc : String
        , notFoundImgSrc : String
        , csrfToken : Concourse.CSRFToken
        , authToken : String
        , pipelineRunningKeyframes : String
        }
