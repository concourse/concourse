module SideBar.SideBar exposing
    ( Model
    , hamburgerMenu
    , handleCallback
    , handleDelivery
    , view
    )

import Concourse
import EffectTransformer exposing (ET)
import Html exposing (Html)
import Html.Attributes exposing (href, id, title)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import List.Extra
import Message.Callback exposing (Callback(..))
import Message.Message exposing (DomID(..), Message(..))
import Message.Subscription exposing (Delivery(..))
import RemoteData exposing (RemoteData(..), WebData)
import Routes
import ScreenSize exposing (ScreenSize(..))
import Set exposing (Set)
import SideBar.Styles as Styles
import Views.Icon as Icon


type alias Model m =
    { m
        | expandedTeams : Set String
        , pipelines : WebData (List Concourse.Pipeline)
        , hovered : Maybe DomID
        , isSideBarOpen : Bool
        , screenSize : ScreenSize.ScreenSize
    }


type alias PipelineScoped a =
    { a
        | teamName : String
        , pipelineName : String
    }


handleCallback : Callback -> WebData (PipelineScoped a) -> ET (Model m)
handleCallback callback currentPipeline ( model, effects ) =
    case callback of
        PipelinesFetched (Ok pipelines) ->
            ( { model
                | pipelines = Success pipelines
                , expandedTeams =
                    case ( model.pipelines, currentPipeline ) of
                        ( NotAsked, Success { teamName } ) ->
                            Set.insert teamName model.expandedTeams

                        _ ->
                            model.expandedTeams
              }
            , effects
            )

        BuildFetched (Ok ( _, build )) ->
            ( { model
                | expandedTeams =
                    case ( currentPipeline, build.job ) of
                        ( NotAsked, Just { teamName } ) ->
                            Set.insert teamName model.expandedTeams

                        _ ->
                            model.expandedTeams
              }
            , effects
            )

        _ ->
            ( model, effects )


handleDelivery : Delivery -> ET (Model m)
handleDelivery delivery ( model, effects ) =
    case delivery of
        SideBarStateReceived (Just "true") ->
            ( { model | isSideBarOpen = True }, effects )

        _ ->
            ( model, effects )


view : Model m -> Maybe (PipelineScoped a) -> Html Message
view model currentPipeline =
    if
        model.isSideBarOpen
            && not
                (RemoteData.map List.isEmpty model.pipelines
                    |> RemoteData.withDefault True
                )
            && (model.screenSize /= ScreenSize.Mobile)
    then
        Html.div
            (id "side-bar" :: Styles.sideBar)
            (model.pipelines
                |> RemoteData.withDefault []
                |> List.Extra.gatherEqualsBy .teamName
                |> List.map
                    (\( p, ps ) ->
                        team
                            { hovered = model.hovered
                            , isExpanded = Set.member p.teamName model.expandedTeams
                            , teamName = p.teamName
                            , pipelines = p :: ps
                            , currentPipeline = currentPipeline
                            }
                    )
            )

    else
        Html.text ""


team :
    { a
        | hovered : Maybe DomID
        , isExpanded : Bool
        , teamName : String
        , pipelines : List Concourse.Pipeline
        , currentPipeline : Maybe (PipelineScoped b)
    }
    -> Html Message
team ({ isExpanded, pipelines } as session) =
    Html.div
        Styles.team
        [ teamHeader session
        , if isExpanded then
            Html.div Styles.column <| List.map (pipeline session) pipelines

          else
            Html.text ""
        ]


teamHeader :
    { a
        | hovered : Maybe DomID
        , isExpanded : Bool
        , teamName : String
        , currentPipeline : Maybe (PipelineScoped b)
    }
    -> Html Message
teamHeader { hovered, isExpanded, teamName, currentPipeline } =
    let
        isHovered =
            hovered == Just (SideBarTeam teamName)

        isCurrent =
            (currentPipeline
                |> Maybe.map .teamName
            )
                == Just teamName
    in
    Html.div
        (Styles.teamHeader
            ++ [ onClick <| Click <| SideBarTeam teamName
               , onMouseEnter <| Hover <| Just <| SideBarTeam teamName
               , onMouseLeave <| Hover Nothing
               ]
        )
        [ Styles.teamIcon { isCurrent = isCurrent, isHovered = isHovered }
        , Styles.arrow
            { isHovered = isHovered
            , isExpanded = isExpanded
            }
        , Html.div
            (title teamName
                :: Styles.teamName
                    { isHovered = isHovered
                    , isCurrent = isCurrent
                    }
            )
            [ Html.text teamName ]
        ]


pipeline :
    { a
        | hovered : Maybe DomID
        , teamName : String
        , currentPipeline : Maybe (PipelineScoped b)
    }
    -> Concourse.Pipeline
    -> Html Message
pipeline { hovered, teamName, currentPipeline } p =
    let
        pipelineId =
            { pipelineName = p.name
            , teamName = teamName
            }

        isCurrent =
            case currentPipeline of
                Just cp ->
                    cp.pipelineName == p.name && cp.teamName == teamName

                Nothing ->
                    False

        isHovered =
            hovered == Just (SideBarPipeline pipelineId)
    in
    Html.div Styles.pipeline
        [ Html.div
            (Styles.pipelineIcon
                { isCurrent = isCurrent
                , isHovered = isHovered
                }
            )
            []
        , Html.a
            (Styles.pipelineLink
                { isHovered = isHovered
                , isCurrent = isCurrent
                }
                ++ [ href <|
                        Routes.toString <|
                            Routes.Pipeline { id = pipelineId, groups = [] }
                   , title p.name
                   , onMouseEnter <| Hover <| Just <| SideBarPipeline pipelineId
                   , onMouseLeave <| Hover Nothing
                   ]
            )
            [ Html.text p.name ]
        ]


hamburgerMenu :
    { a
        | screenSize : ScreenSize
        , pipelines : WebData (List Concourse.Pipeline)
        , isSideBarOpen : Bool
        , hovered : Maybe DomID
    }
    -> Html Message
hamburgerMenu model =
    if model.screenSize == Mobile then
        Html.text ""

    else
        let
            isHamburgerClickable =
                RemoteData.map (not << List.isEmpty) model.pipelines
                    |> RemoteData.withDefault False
        in
        Html.div
            (id "hamburger-menu"
                :: Styles.hamburgerMenu
                    { isSideBarOpen = model.isSideBarOpen
                    , isClickable = isHamburgerClickable
                    }
                ++ [ onMouseEnter <| Hover <| Just HamburgerMenu
                   , onMouseLeave <| Hover Nothing
                   ]
                ++ (if isHamburgerClickable then
                        [ onClick <| Click HamburgerMenu ]

                    else
                        []
                   )
            )
            [ Icon.icon
                { sizePx = 54, image = "baseline-menu-24px.svg" }
              <|
                (Styles.hamburgerIcon <|
                    { isHovered =
                        isHamburgerClickable
                            && (model.hovered == Just HamburgerMenu)
                    , isActive = model.isSideBarOpen
                    }
                )
            ]
