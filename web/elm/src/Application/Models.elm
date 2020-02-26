module Application.Models exposing (Session)

import Concourse
import SideBar.SideBar as SideBar
import Time
import UserState exposing (UserState)


type alias Session =
    SideBar.Model
        { userState : UserState
        , clusterName : String
        , version : String
        , turbulenceImgSrc : String
        , notFoundImgSrc : String
        , csrfToken : Concourse.CSRFToken
        , authToken : String
        , pipelineRunningKeyframes : String
        , timeZone : Time.Zone
        }
