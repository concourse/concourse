module Application.Models exposing (Session)

import Concourse
import UserState exposing (UserState)


type alias Session =
    { turbulenceImgSrc : String
    , notFoundImgSrc : String
    , csrfToken : Concourse.CSRFToken
    , authToken : String
    , pipelineRunningKeyframes : String
    , userState : UserState
    }
