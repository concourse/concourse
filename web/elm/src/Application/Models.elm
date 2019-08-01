module Application.Models exposing (Session)

import Concourse
import HoverState
import SideBar.SideBar as SideBar
import Time
import UserState exposing (UserState)


type alias Session =
    SideBar.Model
        { userState : UserState
        , hovered : HoverState.HoverState
        , clusterName : String
        , turbulenceImgSrc : String
        , notFoundImgSrc : String
        , csrfToken : Concourse.CSRFToken
        , authToken : String
        , pipelineRunningKeyframes : String
        , timeZone : Time.Zone
        }
