module Common exposing
    ( and
    , contains
    , defineHoverBehaviour
    , given
    , iOpenTheBuildPage
    , init
    , isColorWithStripes
    , leftClickEvent
    , myBrowserFetchedTheBuild
    , notContains
    , pipelineRunningKeyframes
    , queryView
    , then_
    , when
    )

import Application.Application as Application
import Concourse
import Concourse.BuildStatus exposing (BuildStatus(..))
import Expect exposing (Expectation)
import Html
import Json.Encode
import List.Extra
import Message.Callback as Callback
import Message.Effects exposing (Effect)
import Message.Message exposing (DomID, Message(..))
import Message.TopLevelMessage exposing (TopLevelMessage(..))
import Test exposing (Test, describe, test)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector exposing (Selector, style)
import Url


queryView : Application.Model -> Query.Single TopLevelMessage
queryView =
    Application.view
        >> .body
        >> Html.div []
        >> Query.fromHtml


contains : a -> List a -> Expect.Expectation
contains x xs =
    if List.member x xs then
        Expect.pass

    else
        Expect.fail <|
            "Expected \n[ "
                ++ String.join "\n, " (List.map Debug.toString xs)
                ++ "\n] to contain "
                ++ Debug.toString x


notContains : a -> List a -> Expect.Expectation
notContains x xs =
    if List.member x xs then
        Expect.fail <|
            "Expected "
                ++ Debug.toString xs
                ++ " not to contain "
                ++ Debug.toString x

    else
        Expect.pass


leftClickEvent : ( String, Json.Encode.Value )
leftClickEvent =
    Event.custom "click" <|
        Json.Encode.object
            [ ( "ctrlKey", Json.Encode.bool False )
            , ( "altKey", Json.Encode.bool False )
            , ( "metaKey", Json.Encode.bool False )
            , ( "shiftKey", Json.Encode.bool False )
            , ( "button", Json.Encode.int 0 )
            ]


isColorWithStripes :
    { thick : String, thin : String }
    -> Query.Single msg
    -> Expectation
isColorWithStripes { thick, thin } =
    Query.has
        [ style "background-image" <|
            "repeating-linear-gradient(-115deg,"
                ++ thin
                ++ " 0px,"
                ++ thick
                ++ " 1px,"
                ++ thick
                ++ " 10px,"
                ++ thin
                ++ " 11px,"
                ++ thin
                ++ " 16px)"
        , style "background-size" "106px 114px"
        , style "animation" <|
            pipelineRunningKeyframes
                ++ " 3s linear infinite"
        ]


pipelineRunningKeyframes : String
pipelineRunningKeyframes =
    "pipeline-running"


init : String -> Application.Model
init path =
    Application.init
        { turbulenceImgSrc = ""
        , notFoundImgSrc = "notfound.svg"
        , csrfToken = "csrf_token"
        , authToken = ""
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


given =
    identity


and =
    identity


when =
    identity


then_ =
    identity


iOpenTheBuildPage _ =
    Application.init
        { turbulenceImgSrc = ""
        , notFoundImgSrc = ""
        , csrfToken = ""
        , authToken = ""
        , pipelineRunningKeyframes = ""
        }
        { protocol = Url.Http
        , host = ""
        , port_ = Nothing
        , path = "/builds/1"
        , query = Nothing
        , fragment = Nothing
        }


myBrowserFetchedTheBuild =
    Tuple.first
        >> Application.handleCallback
            (Callback.BuildFetched <|
                Ok
                    { id = 1
                    , name = "1"
                    , job =
                        Just
                            { teamName = "other-team"
                            , pipelineName = "yet-another-pipeline"
                            , jobName = "job"
                            }
                    , status = BuildStatusStarted
                    , duration =
                        { startedAt = Nothing
                        , finishedAt = Nothing
                        }
                    , reapTime = Nothing
                    }
            )


defineHoverBehaviour :
    { name : String
    , setup : Application.Model
    , query : Application.Model -> Query.Single TopLevelMessage
    , unhoveredSelector : { description : String, selector : List Selector }
    , hoverable : DomID
    , hoveredSelector : { description : String, selector : List Selector }
    }
    -> Test
defineHoverBehaviour { name, setup, query, unhoveredSelector, hoverable, hoveredSelector } =
    describe (name ++ " hover behaviour")
        [ test (name ++ " is " ++ unhoveredSelector.description) <|
            \_ ->
                setup
                    |> query
                    |> Query.has unhoveredSelector.selector
        , test ("mousing over " ++ name ++ " triggers Hover msg") <|
            \_ ->
                setup
                    |> query
                    |> Event.simulate Event.mouseEnter
                    |> Event.expect (Update <| Hover <| Just hoverable)
        , test
            ("Hover msg causes "
                ++ name
                ++ " to become "
                ++ hoveredSelector.description
            )
          <|
            \_ ->
                setup
                    |> Application.update (Update <| Hover <| Just hoverable)
                    |> Tuple.first
                    |> query
                    |> Query.has hoveredSelector.selector
        , test ("mousing off " ++ name ++ " triggers unhover msg") <|
            \_ ->
                setup
                    |> Application.update (Update <| Hover <| Just hoverable)
                    |> Tuple.first
                    |> query
                    |> Event.simulate Event.mouseLeave
                    |> Event.expect (Update <| Hover Nothing)
        , test
            ("unhover msg causes "
                ++ name
                ++ " to become "
                ++ unhoveredSelector.description
            )
          <|
            \_ ->
                setup
                    |> Application.update (Update <| Hover <| Just hoverable)
                    |> Tuple.first
                    |> Application.update (Update <| Hover Nothing)
                    |> Tuple.first
                    |> query
                    |> Query.has unhoveredSelector.selector
        ]



-- 6 places where Application.init is used with a query
-- 6 places where Application.init is used with a fragment
-- 1 place where Application.init is used with an instance name
