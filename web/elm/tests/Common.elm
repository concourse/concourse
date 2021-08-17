module Common exposing
    ( and
    , contains
    , defineHoverBehaviour
    , expectNoTooltip
    , expectTooltip
    , expectTooltipWith
    , given
    , givenDataUnauthenticated
    , gotPipelines
    , hoverOver
    , iOpenTheBuildPage
    , iOpenTheBuildPageHighlighting
    , init
    , initCustom
    , initCustomOpts
    , initRoute
    , isColorWithStripes
    , myBrowserFetchedTheBuild
    , notContains
    , pipelineRunningKeyframes
    , queryView
    , routeHref
    , then_
    , when
    , whenOnDesktop
    , whenOnMobile
    , withAllPipelinesVisible
    )

import Application.Application as Application
import Concourse
import Concourse.BuildStatus exposing (BuildStatus(..))
import Data
import EffectTransformer exposing (ET)
import Expect exposing (Expectation)
import Html
import Html.Attributes as Attr
import List.Extra
import Message.Callback as Callback
import Message.Effects exposing (Effect)
import Message.Message exposing (DomID(..), Message(..))
import Message.Subscription exposing (Delivery(..))
import Message.TopLevelMessage exposing (TopLevelMessage(..))
import Routes
import Test exposing (Test, describe, test)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector exposing (Selector, attribute, id, style, text)
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


routeHref : Routes.Route -> Test.Html.Selector.Selector
routeHref =
    Routes.toString >> Attr.href >> attribute


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


type alias InitCustomOpts =
    { query : Maybe String
    , fragment : Maybe String
    , featureFlags : Concourse.FeatureFlags
    }


initCustomOpts : InitCustomOpts
initCustomOpts =
    { query = Nothing
    , fragment = Nothing
    , featureFlags = Data.featureFlags
    }


initCustom : InitCustomOpts -> String -> Application.Model
initCustom { query, featureFlags } path =
    let
        flags =
            Data.flags
    in
    Application.init { flags | featureFlags = featureFlags }
        { protocol = Url.Http
        , host = ""
        , port_ = Nothing
        , path = path
        , query = query
        , fragment = Nothing
        }
        |> Tuple.first


initRoute : Routes.Route -> Application.Model
initRoute route =
    case Url.fromString ("http://test.com" ++ Routes.toString route) of
        Just url ->
            Application.init Data.flags url |> Tuple.first

        Nothing ->
            Debug.todo ("invalid route stringification: " ++ Debug.toString route)


init : String -> Application.Model
init =
    initCustom initCustomOpts


given =
    identity


and =
    identity


when =
    identity


then_ =
    identity


iOpenTheBuildPage : a -> ( Application.Model, List Effect )
iOpenTheBuildPage _ =
    Application.init Data.flags
        { protocol = Url.Http
        , host = ""
        , port_ = Nothing
        , path = "/builds/1"
        , query = Nothing
        , fragment = Nothing
        }


iOpenTheBuildPageHighlighting : String -> a -> ( Application.Model, List Effect )
iOpenTheBuildPageHighlighting stepId _ =
    Application.init Data.flags
        { protocol = Url.Http
        , host = ""
        , port_ = Nothing
        , path = "/builds/1"
        , query = Nothing
        , fragment = Just <| "L" ++ stepId ++ ":1"
        }


myBrowserFetchedTheBuild =
    Tuple.first
        >> Application.handleCallback
            (Callback.BuildFetched <|
                Ok
                    { id = 1
                    , name = "1"
                    , teamName = "other-team"
                    , job =
                        Just
                            (Data.jobId
                                |> Data.withTeamName "other-team"
                                |> Data.withPipelineName "yet-another-pipeline"
                                |> Data.withJobName "job"
                            )
                    , status = BuildStatusStarted
                    , duration =
                        { startedAt = Nothing
                        , finishedAt = Nothing
                        }
                    , reapTime = Nothing
                    , createdBy = Nothing
                    , comment = ""
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


withScreenSize : Float -> Float -> Application.Model -> Application.Model
withScreenSize width height =
    Application.handleCallback
        (Callback.ScreenResized
            { scene = { width = 0, height = 0 }
            , viewport = { x = 0, y = 0, width = width, height = height }
            }
        )
        >> Tuple.first


withAllPipelinesVisible : Application.Model -> Application.Model
withAllPipelinesVisible =
    Application.handleCallback
        (Callback.GotViewport
            Dashboard
            (Ok
                { scene = { width = 0, height = 0 }
                , viewport = { x = 0, y = 0, width = 1000, height = 100000 }
                }
            )
        )
        >> Tuple.first


whenOnDesktop : Application.Model -> Application.Model
whenOnDesktop =
    withScreenSize 1500 900


whenOnMobile : Application.Model -> Application.Model
whenOnMobile =
    withScreenSize 400 900


givenDataUnauthenticated :
    List Concourse.Team
    -> Application.Model
    -> ( Application.Model, List Effect )
givenDataUnauthenticated data =
    Application.handleCallback
        (Callback.AllTeamsFetched <| Ok data)
        >> Tuple.first
        >> Application.handleCallback
            (Callback.UserFetched <| Data.httpUnauthorized)


gotPipelines : List ( Concourse.Pipeline, List Concourse.Job ) -> Application.Model -> Application.Model
gotPipelines data =
    let
        pipelines =
            data |> List.map Tuple.first

        jobs =
            data |> List.concatMap Tuple.second

        teams =
            pipelines |> List.map .teamName |> List.Extra.unique |> List.indexedMap Concourse.Team
    in
    Application.handleCallback
        (Callback.AllPipelinesFetched <| Ok pipelines)
        >> Tuple.first
        >> Application.handleCallback
            (Callback.AllJobsFetched <| Ok jobs)
        >> Tuple.first
        >> givenDataUnauthenticated teams
        >> Tuple.first


hoverOver : Message.Message.DomID -> Application.Model -> ( Application.Model, List Effect )
hoverOver domID =
    Application.update
        (Update (Message.Message.Hover (Just domID)))
        >> Tuple.first
        >> Application.handleCallback
            (Callback.GotViewport domID <|
                Ok
                    { scene =
                        { width = 1
                        , height = 0
                        }
                    , viewport =
                        { width = 1
                        , height = 0
                        , x = 0
                        , y = 0
                        }
                    }
            )
        >> Tuple.first
        >> Application.handleCallback
            (Callback.GotElement <|
                Ok
                    { scene =
                        { width = 0
                        , height = 0
                        }
                    , viewport =
                        { width = 0
                        , height = 0
                        , x = 0
                        , y = 0
                        }
                    , element =
                        { x = 0
                        , y = 0
                        , width = 1
                        , height = 1
                        }
                    }
            )


expectTooltip : DomID -> String -> Application.Model -> Expect.Expectation
expectTooltip domID tooltip =
    hoverOver domID
        >> Tuple.first
        >> queryView
        >> Query.find [ id "tooltips" ]
        >> Query.has [ text tooltip ]


expectTooltipWith : DomID -> (Query.Single TopLevelMessage -> Expect.Expectation) -> Application.Model -> Expect.Expectation
expectTooltipWith domID expectation =
    hoverOver domID
        >> Tuple.first
        >> queryView
        >> Query.find [ id "tooltips" ]
        >> expectation


expectNoTooltip : DomID -> Application.Model -> Expect.Expectation
expectNoTooltip domID =
    hoverOver domID
        >> Tuple.first
        >> queryView
        >> Query.hasNot [ id "tooltips" ]
