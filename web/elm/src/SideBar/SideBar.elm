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
import Html.Attributes exposing (id)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import List.Extra
import Message.Callback exposing (Callback(..))
import Message.Message exposing (DomID(..), Message(..))
import Message.Subscription exposing (Delivery(..))
import RemoteData exposing (RemoteData(..), WebData)
import ScreenSize exposing (ScreenSize(..))
import Set exposing (Set)
import SideBar.Styles as Styles
import SideBar.Team as Team
import SideBar.Views as Views
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
                        Team.team
                            { hovered = model.hovered
                            , pipelines = p :: ps
                            , currentPipeline = currentPipeline
                            }
                            { name = p.teamName
                            , isExpanded = Set.member p.teamName model.expandedTeams
                            }
                            |> Views.viewTeam
                    )
            )

    else
        Html.text ""


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
