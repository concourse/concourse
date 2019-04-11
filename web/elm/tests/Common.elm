module Common exposing (init, queryView)

import Application.Application as Application
import Html
import Message.Effects exposing (Effect)
import Message.TopLevelMessage exposing (TopLevelMessage)
import Test.Html.Query as Query
import Url


queryView : Application.Model -> Query.Single TopLevelMessage
queryView =
    Application.view
        >> .body
        >> List.head
        >> Maybe.withDefault (Html.text "")
        >> Query.fromHtml


init : String -> Application.Model
init path =
    Application.init
        { turbulenceImgSrc = ""
        , notFoundImgSrc = "notfound.svg"
        , csrfToken = "csrf_token"
        , authToken = ""
        , instanceName = ""
        , pipelineRunningKeyframes = "pipeline-running"
        }
        { protocol = Url.Http
        , host = ""
        , port_ = Nothing
        , path = path
        , query = Nothing
        , fragment = Nothing
        }
        |> Tuple.first



-- 6 places where Application.init is used with a query
-- 6 places where Application.init is used with a fragment
-- 1 place where Application.init is used with an instance name
